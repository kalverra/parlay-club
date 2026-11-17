// Package export turns a locked slip's legs into the sportsbook links a person
// can tap, plus a manual list for every leg the book cannot take from a link.
// It is pure: it reads stored odds and never fetches anything.
package export

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
)

// The books the export offers, spelled as Parlay API keys.
const (
	DraftKings = "draftkings"
	FanDuel    = "fanduel"
)

// Reason says why a leg has to be added to the sportsbook by hand.
type Reason string

// The manual reasons a leg can carry.
const (
	// ReasonNoLink means the book priced the selection but returned no link.
	ReasonNoLink Reason = "no link"
	// ReasonLineMoved means the book's line no longer matches the leg's line.
	ReasonLineMoved Reason = "line moved"
	// ReasonGameStarted means the game kicked off, so the link would not work.
	ReasonGameStarted Reason = "game started"
	// ReasonNoLine means the book has no odds row for the selection at all.
	ReasonNoLine Reason = "no line"
)

// Leg is one slip leg with the kickoff of its game. Kickoff is nil for a leg
// with no game, such as a hand-typed OTHER leg on a past-week slip.
type Leg struct {
	db.Leg
	Kickoff *time.Time
}

// LegLink is a leg the book can open from a link, with the line, price, and
// selection ID the book quoted for the same selection. EventSid is the book's
// own ID for the game, which some combined links need. LineMoved is true when
// the book's current line differs from the line stored on the slip.
type LegLink struct {
	Leg       Leg
	URL       string
	Line      *float64
	Price     int
	Sid       *string
	EventSid  *string
	LineMoved bool
}

// ManualLeg is a leg the person has to add by hand. Line is the book's current
// line when it has one, so a moved line can be shown beside the agreed one.
type ManualLeg struct {
	Leg    Leg
	Reason Reason
	Line   *float64
}

// BookPlan is one book's export of a locked slip. Every leg is in exactly one
// of Linked or Manual. Combined is set only when the book has a verified format
// for one URL that holds several selections.
type BookPlan struct {
	Book     string
	Linked   []LegLink
	Manual   []ManualLeg
	Combined string
}

// Combiner builds one sportsbook URL for several linked legs. A book with no
// verified multi-selection format is absent from the registry, so Plan leaves
// Combined empty and the panel shows one tap per leg.
type Combiner interface {
	Combine(linked []LegLink) (url string, ok bool)
}

// combiners holds each book's verified Combiner, keyed by book. Both formats
// were checked on a phone on 2026-11-07. A book whose format stops working can
// be returned to per-leg taps by removing it here.
var combiners = map[string]Combiner{
	DraftKings: draftKingsCombiner{},
	FanDuel:    fanDuelCombiner{},
}

// Plan builds one book's export for a locked slip. A slip that is not locked
// gets an empty plan, because the export belongs to a finished slip. now is
// injected so the kickoff rule stays testable.
func Plan(slip db.Slip, legs []Leg, odds []db.Odds, book string, now time.Time) BookPlan {
	return PlanWithCombiner(slip, legs, odds, book, now, combiners[db.NormalizeBook(book)])
}

// PlanWithCombiner is Plan with an explicit Combiner. A nil Combiner leaves
// Combined empty and the per-leg links stand alone. Tests use it to inject a
// fake or nil Combiner independently of the default registry.
func PlanWithCombiner(
	slip db.Slip,
	legs []Leg,
	odds []db.Odds,
	book string,
	now time.Time,
	combiner Combiner,
) BookPlan {
	plan := BookPlan{Book: book}
	if slip.Status != db.SlipLocked {
		return plan
	}

	for _, leg := range legs {
		linked, manual, ok := planLeg(leg, odds, book, now)
		if ok {
			plan.Linked = append(plan.Linked, linked)
			continue
		}
		plan.Manual = append(plan.Manual, manual)
	}

	if combiner != nil && len(plan.Linked) > 0 {
		if combined, ok := combiner.Combine(plan.Linked); ok {
			plan.Combined = combined
		}
	}
	return plan
}

// draftKingsEvent is the event-page prefix the phone check confirmed. The root
// ?outcomes= link only ever opened one selection; the event page takes them all.
const draftKingsEvent = "https://sportsbook.draftkings.com/event/"

// draftKingsCombiner builds one betslip URL for two or more DraftKings
// selections from their stored sids. Every leg must belong to the same
// DraftKings event, because the verified format is an event page. Each sid is
// escaped on its own so the comma stays the separator and a prop sid's # becomes
// %23.
type draftKingsCombiner struct{}

// Combine returns DraftKings' event-page URL, or false when every linked leg
// cannot take part.
func (draftKingsCombiner) Combine(linked []LegLink) (string, bool) {
	if len(linked) < 2 {
		return "", false
	}

	eventSid, ok := sharedEventSid(linked)
	if !ok {
		return "", false
	}
	sids := make([]string, 0, len(linked))
	for _, leg := range linked {
		if leg.Sid == nil || strings.TrimSpace(*leg.Sid) == "" {
			return "", false
		}
		sids = append(sids, url.QueryEscape(strings.TrimSpace(*leg.Sid)))
	}
	return draftKingsEvent + url.PathEscape(eventSid) + "?outcomes=" + strings.Join(sids, ","), true
}

// sharedEventSid returns the one event ID every linked leg belongs to. A slip
// that spans events cannot use the event-page form, so it keeps per-leg taps.
func sharedEventSid(linked []LegLink) (string, bool) {
	if len(linked) == 0 || linked[0].EventSid == nil {
		return "", false
	}
	eventSid := strings.TrimSpace(*linked[0].EventSid)
	if eventSid == "" {
		return "", false
	}
	for _, leg := range linked[1:] {
		if leg.EventSid == nil || strings.TrimSpace(*leg.EventSid) != eventSid {
			return "", false
		}
	}
	return eventSid, true
}

// fanDuelBetslip is the endpoint the phone check confirmed. FanDuel names each
// selection's market and selection with an index, and the brackets are
// percent-encoded so a share sheet keeps the whole URL together.
const fanDuelBetslip = "https://sportsbook.fanduel.com/addToBetslip"

// fanDuelCombiner builds one betslip URL for two or more FanDuel selections.
// The market ID lives only in the stored per-leg link, so it is read from
// there; the selection ID comes from the stored sid. If any linked leg lacks
// either, the plan falls back to per-leg taps rather than dropping a pick.
type fanDuelCombiner struct{}

// Combine returns FanDuel's indexed betslip URL, or false when every linked leg
// cannot take part.
func (fanDuelCombiner) Combine(linked []LegLink) (string, bool) {
	if len(linked) < 2 {
		return "", false
	}

	var b strings.Builder
	b.WriteString(fanDuelBetslip)
	separator := "?"
	for i, leg := range linked {
		if leg.Sid == nil || strings.TrimSpace(*leg.Sid) == "" {
			return "", false
		}
		marketID, ok := fanDuelMarketID(leg.URL)
		if !ok {
			return "", false
		}
		b.WriteString(separator)
		separator = "&"
		b.WriteString("marketId%5B")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("%5D=")
		b.WriteString(url.QueryEscape(marketID))
		b.WriteString("&selectionId%5B")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("%5D=")
		b.WriteString(url.QueryEscape(strings.TrimSpace(*leg.Sid)))
	}
	return b.String(), true
}

// fanDuelMarketID pulls the market ID out of the per-leg link Parlay API
// stored. A link without one cannot take part in a combined link.
func fanDuelMarketID(link string) (string, bool) {
	parsed, err := url.Parse(link)
	if err != nil {
		return "", false
	}
	marketID := strings.TrimSpace(parsed.Query().Get("marketId"))
	if marketID == "" {
		return "", false
	}
	return marketID, true
}

// planLeg links one leg or returns the manual entry that explains why it cannot
// be linked. The kickoff check comes first: a started game cannot be linked
// even when its stored row still has a link.
func planLeg(leg Leg, odds []db.Odds, book string, now time.Time) (LegLink, ManualLeg, bool) {
	if leg.GameID == nil {
		return LegLink{}, ManualLeg{Leg: leg, Reason: ReasonNoLine}, false
	}
	if leg.Kickoff != nil && !now.Before(*leg.Kickoff) {
		return LegLink{}, ManualLeg{Leg: leg, Reason: ReasonGameStarted}, false
	}

	row, found := matchOdds(leg.Leg, odds, book)
	if !found {
		return LegLink{}, ManualLeg{Leg: leg, Reason: ReasonNoLine}, false
	}
	if row.Link == nil || strings.TrimSpace(*row.Link) == "" {
		return LegLink{}, ManualLeg{Leg: leg, Reason: ReasonNoLink}, false
	}

	return LegLink{
		Leg:       leg,
		URL:       *row.Link,
		Line:      row.Line,
		Price:     row.Price,
		Sid:       row.Sid,
		EventSid:  row.EventSid,
		LineMoved: !db.SameLine(row.Line, leg.Line),
	}, ManualLeg{}, true
}

// matchOdds finds the odds row for a leg's selection and book via
// MatchesSelection (SameSelection), preferring an exact line match when more
// than one row matches.
func matchOdds(leg db.Leg, odds []db.Odds, book string) (db.Odds, bool) {
	book = db.NormalizeBook(book)
	var fallback db.Odds
	var found bool
	for _, row := range odds {
		if db.NormalizeBook(row.Book) != book {
			continue
		}
		if !row.MatchesSelection(leg) {
			continue
		}
		if db.SameLine(row.Line, leg.Line) {
			return row, true
		}
		if !found {
			fallback = row
			found = true
		}
	}
	return fallback, found
}
