package web

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/export"
)

// exportBooks are the books the export card offers, in the order it draws
// them. The API keys come from internal/export.
var exportBooks = []struct {
	key   string
	label string
}{
	{export.DraftKings, "DraftKings"},
	{export.FanDuel, "FanDuel"},
}

// ExportView is the export card on a locked slip. Locked is false on an open
// slip, where the card draws only its empty refresh wrapper.
type ExportView struct {
	SlipID       int64
	Locked       bool
	Books        []ExportBookView
	LinesAtISO   string
	LinesAtLabel string
	CopyText     string
}

// ExportBookView is one sportsbook's part of the card: one combined link when
// the book has a verified format, otherwise one tap per linked leg, plus every
// leg the person has to add by hand.
type ExportBookView struct {
	Label   string
	OpenURL string
	Linked  []ExportLinkView
	Manual  []ExportManualView
}

// ExportLinkView is one linked leg's tap.
type ExportLinkView struct {
	Selection string
	URL       string
	Price     string
	LineMoved bool
	DriftNote string
}

// ExportManualView is one manual leg and the friendly note that says why.
type ExportManualView struct {
	Selection string
	Note      string
}

// export returns the export card partial for SSE refreshes.
func (h *slipHandlers) export(w http.ResponseWriter, r *http.Request) {
	slip, _, _, ok := h.loadSlip(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	view, err := h.exportView(r.Context(), slip)
	if err != nil {
		h.serverError(w, err)
		return
	}
	h.renderPartial(w, "slip-export", view)
}

// exportView loads the slip's legs, their kickoffs, and the odds rows for each
// game, then plans both books from what is stored. It never fetches odds and
// never writes.
func (h *slipHandlers) exportView(ctx context.Context, slip db.Slip) (ExportView, error) {
	view := ExportView{SlipID: slip.ID, Locked: slip.Status == db.SlipLocked}
	if !view.Locked {
		return view, nil
	}

	members, err := h.deps.Store.SlipMembers(ctx, slip.ID)
	if err != nil {
		return ExportView{}, err
	}
	legs, err := h.deps.Store.LegsForSlip(ctx, slip.ID)
	if err != nil {
		return ExportView{}, err
	}
	games, err := h.deps.Store.GamesForWeek(ctx, slip.WeekID)
	if err != nil {
		return ExportView{}, err
	}

	kickoffs := make(map[string]time.Time, len(games))
	for _, game := range games {
		kickoffs[game.ID] = game.Kickoff
	}

	oddsByGame := make(map[string][]db.Odds, len(kickoffs))
	oddsRows := make([]db.Odds, 0)
	var newest time.Time
	for _, leg := range legs {
		if leg.GameID == nil {
			continue
		}
		if _, seen := oddsByGame[*leg.GameID]; !seen {
			rows, err := h.deps.Store.OddsForGame(ctx, *leg.GameID)
			if err != nil {
				return ExportView{}, err
			}
			rows = currentOdds(rows)
			oddsByGame[*leg.GameID] = rows
			oddsRows = append(oddsRows, rows...)
			for _, row := range rows {
				if row.FetchedAt.After(newest) {
					newest = row.FetchedAt
				}
			}
		}
	}

	names := make(map[int64]string, len(members))
	for _, member := range members {
		names[member.User.ID] = member.User.Name
	}

	exportLegs := make([]export.Leg, 0, len(legs))
	for _, leg := range legs {
		item := export.Leg{Leg: leg}
		if leg.GameID != nil {
			if kickoff, found := kickoffs[*leg.GameID]; found {
				item.Kickoff = &kickoff
			}
		}
		exportLegs = append(exportLegs, item)
	}

	now := h.deps.now()
	for _, book := range exportBooks {
		plan := export.Plan(slip, exportLegs, oddsRows, book.key, now)
		view.Books = append(view.Books, exportBookView(plan, book.label))
	}

	view.CopyText = exportCopyText(legs, names)
	view.LinesAtISO, view.LinesAtLabel = linksNote(newest)
	return view, nil
}

// exportBookView shapes one book's plan and turns each manual reason into a
// short note in the app's voice.
func exportBookView(plan export.BookPlan, label string) ExportBookView {
	book := ExportBookView{Label: label, OpenURL: plan.Combined}
	for _, link := range plan.Linked {
		book.Linked = append(book.Linked, ExportLinkView{
			Selection: link.Leg.Selection,
			URL:       link.URL,
			Price:     formatOdds(link.Price),
			LineMoved: link.LineMoved,
			DriftNote: driftNote(link),
		})
	}
	for _, manual := range plan.Manual {
		book.Manual = append(book.Manual, ExportManualView{
			Selection: manual.Leg.Selection,
			Note:      manualNote(manual, label),
		})
	}
	return book
}

// driftNote formats a concise line-drift callout when a linked leg's live
// sportsbook line differs from the line stored on the slip.
func driftNote(link export.LegLink) string {
	if !link.LineMoved {
		return ""
	}
	bookLine := formatExportLine(link.Leg.MarketType, link.Line)
	slipLine := formatExportLine(link.Leg.MarketType, link.Leg.Line)
	if bookLine != "" && slipLine != "" {
		return fmt.Sprintf("Book at %s · slip %s", bookLine, slipLine)
	}
	if bookLine != "" {
		return "Book at " + bookLine
	}
	return "Line moved"
}

// formatExportLine formats an optional market line for display, keeping signs on
// spreads while omitting them on totals and props.
func formatExportLine(marketType string, line *float64) string {
	if line == nil {
		return ""
	}
	if marketType == "SPREAD" {
		return signedLine(*line)
	}
	return formatFloat(*line)
}

// manualNote says why a leg needs a hand, in plain words. It never blocks the
// person and never blames them.
func manualNote(manual export.ManualLeg, book string) string {
	switch manual.Reason {
	case export.ReasonNoLink:
		return fmt.Sprintf(
			"Heads up: this pick didn't come with a %s link, so add it by hand.",
			book,
		)
	case export.ReasonLineMoved:
		if line := formatExportLine(manual.Leg.MarketType, manual.Line); line != "" {
			return fmt.Sprintf(
				"Heads up: %s moved this line to %s, so add it by hand at your line.",
				book,
				line,
			)
		}
		return fmt.Sprintf(
			"Heads up: %s moved this line, so add it by hand at your line.",
			book,
		)
	case export.ReasonGameStarted:
		return "Heads up: this game already started, so add it by hand."
	case export.ReasonNoLine:
		return fmt.Sprintf(
			"Heads up: %s has no line for this pick yet, so add it by hand.",
			book,
		)
	default:
		return "Heads up: add this pick by hand."
	}
}

// exportCopyText writes the whole slip as plain text: one line per leg with the
// member, the selection, and the price at lock.
func exportCopyText(legs []db.Leg, names map[int64]string) string {
	lines := make([]string, 0, len(legs))
	for _, leg := range legs {
		name := strings.TrimSpace(names[leg.UserID])
		if name == "" {
			name = "Someone"
		}
		lines = append(lines, fmt.Sprintf("%s: %s (%s)", name, leg.Selection, formatOdds(leg.Odds)))
	}
	return strings.Join(lines, "\n")
}

// linksNote is the "prices may have moved" note the export card shows. It reads
// the newest fetch time among the slip games' odds rows, the same clock the
// Games tab uses.
func linksNote(newest time.Time) (string, string) {
	if newest.IsZero() {
		return "", ""
	}
	newest = newest.UTC()
	return newest.Format(time.RFC3339), newest.Format("3:04 PM") + " UTC"
}
