package sandbox

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kalverra/parlay-club/internal/auth"
	"github.com/kalverra/parlay-club/internal/db"
	"github.com/kalverra/parlay-club/internal/slips"
)

// PersonaID is the stable name of one seeded person. Stages and tests refer to
// people by these names instead of the numeric row IDs a reset creates.
type PersonaID string

// The seeded people. Ada, Ben, and Cleo share one group; Dex stays outside it
// so the normal invite and 404 rules can be exercised.
const (
	PersonaAda  PersonaID = "ada"
	PersonaBen  PersonaID = "ben"
	PersonaCleo PersonaID = "cleo"
	PersonaDex  PersonaID = "dex"
)

// LegID is the stable name of one seeded pick. Later stages grade these picks
// through the ordinary slip service, so a reset can rename the numeric ID
// without losing the script.
type LegID string

// The seeded picks, two per shared persona.
const (
	LegAdaBengalsML     LegID = "ada-bengals-ml"
	LegAdaChiefsSpread  LegID = "ada-chiefs-spread"
	LegBenEaglesML      LegID = "ben-eagles-ml"
	LegBenRamsSpread    LegID = "ben-rams-spread"
	LegCleoFeaturedOver LegID = "cleo-featured-over"
	LegCleoRamsOver     LegID = "cleo-rams-over"
)

const (
	// SharedGroupName names the one group the shared personas belong to.
	SharedGroupName = "Sunday Sandbox"
	slipName        = "Week 9 Sandbox"
	legsPerMember   = 2
)

// Persona is one fixed sign-in identity. Emails end in .test so they can never
// look like a real address, and Shared says whether the person belongs to the
// shared sandbox group.
type Persona struct {
	ID     PersonaID
	Name   string
	Email  string
	Shared bool
}

// SeededPerson is a persona after a reset: the real user row and the group
// cookie the browser should carry for them.
type SeededPerson struct {
	Persona Persona
	User    db.User
	// GroupID is the shared group for shared personas and the outsider's own
	// group for Dex.
	GroupID int64
}

// Seed is everything a reset created, named by the symbolic IDs later stages
// use. It is returned by the controller and never persisted.
type Seed struct {
	Week          db.Week
	SharedGroup   db.Group
	OutsiderGroup db.Group
	Slip          db.Slip
	People        []SeededPerson
	Legs          map[LegID]int64
}

// Person returns the seeded person with the symbolic ID.
func (s Seed) Person(id PersonaID) (SeededPerson, bool) {
	for _, person := range s.People {
		if person.Persona.ID == id {
			return person, true
		}
	}
	return SeededPerson{}, false
}

// PersonByEmail returns the seeded person whose fixed email matches. Sign-in
// normalizes email case, so the comparison ignores it too.
func (s Seed) PersonByEmail(email string) (SeededPerson, bool) {
	for _, person := range s.People {
		if strings.EqualFold(person.User.Email, email) {
			return person, true
		}
	}
	return SeededPerson{}, false
}

// SharedPersonas returns the three people in the shared group in their fixed
// order.
func SharedPersonas() []Persona {
	return []Persona{
		{ID: PersonaAda, Name: "Ada", Email: "ada@sandbox.test", Shared: true},
		{ID: PersonaBen, Name: "Ben", Email: "ben@sandbox.test", Shared: true},
		{ID: PersonaCleo, Name: "Cleo", Email: "cleo@sandbox.test", Shared: true},
	}
}

// Outsider returns the person who stays out of the shared group.
func Outsider() Persona {
	return Persona{ID: PersonaDex, Name: "Dex", Email: "dex@sandbox.test"}
}

// Personas returns every seeded person, shared members first.
func Personas() []Persona {
	people := SharedPersonas()
	return append(people, Outsider())
}

// PersonaByID returns the persona with the symbolic ID used by the controls
// and the stage manifests.
func PersonaByID(id PersonaID) (Persona, bool) {
	for _, persona := range Personas() {
		if persona.ID == id {
			return persona, true
		}
	}
	return Persona{}, false
}

// PersonaGroup returns the group a persona should act in: the shared sandbox
// group for a shared member, or their own group for the outsider. It reads the
// live memberships, so it works after a restart that did not rebuild the seed.
func PersonaGroup(
	ctx context.Context,
	store *db.Store,
	persona Persona,
	userID int64,
) (db.Group, error) {
	memberships, err := store.ForUser(ctx, userID)
	if err != nil {
		return db.Group{}, fmt.Errorf("persona %s groups: %w", persona.ID, err)
	}
	if len(memberships) == 0 {
		return db.Group{}, fmt.Errorf("persona %s belongs to no group", persona.ID)
	}

	if persona.Shared {
		for _, group := range memberships {
			if group.Name == SharedGroupName {
				return group, nil
			}
		}
		return db.Group{}, fmt.Errorf("persona %s: no %q group", persona.ID, SharedGroupName)
	}

	// The outsider keeps their own group even after accepting an invite, so the
	// controls can always return them to the isolated view.
	for _, group := range memberships {
		if group.Name != SharedGroupName {
			return group, nil
		}
	}
	return memberships[0], nil
}

// LegDefinition is one scripted pick before it gets a numeric leg ID.
type LegDefinition struct {
	ID         LegID
	Persona    PersonaID
	GameID     string
	MarketType string
	Selection  string
	Team       *string
	Side       *string
	Line       *float64
	Odds       int
}

// LegDefinitions returns the picks every morning starts with. They use games
// from the week 9 fixture and prices already present in the line board, so the
// slip remains editable and every pick stays pending until a stage grades it.
func LegDefinitions() []LegDefinition {
	return []LegDefinition{
		{
			ID: LegAdaBengalsML, Persona: PersonaAda,
			GameID: FeaturedGameID, MarketType: "ML", Selection: "CIN ML",
			Team: new("CIN"), Side: new("TEAM"), Odds: -175,
		},
		{
			ID: LegAdaChiefsSpread, Persona: PersonaAda,
			GameID: "401873045", MarketType: "SPREAD", Selection: "KC -9.5",
			Team: new("KC"), Side: new("TEAM"), Line: new(-9.5), Odds: -112,
		},
		{
			ID: LegBenEaglesML, Persona: PersonaBen,
			GameID: "401873044", MarketType: "ML", Selection: "PHI ML",
			Team: new("PHI"), Side: new("TEAM"), Odds: 136,
		},
		{
			ID: LegBenRamsSpread, Persona: PersonaBen,
			GameID: "401873043", MarketType: "SPREAD", Selection: "LAR -4.5",
			Team: new("LAR"), Side: new("TEAM"), Line: new(-4.5), Odds: -120,
		},
		{
			ID: LegCleoFeaturedOver, Persona: PersonaCleo,
			GameID: FeaturedGameID, MarketType: "TOTAL", Selection: "Over 52.5",
			Side: new("OVER"), Line: new(52.5), Odds: -108,
		},
		{
			ID: LegCleoRamsOver, Persona: PersonaCleo,
			GameID: "401873043", MarketType: "TOTAL", Selection: "Over 47.5",
			Side: new("OVER"), Line: new(47.5), Odds: -112,
		},
	}
}

// SeedScenario creates the sandbox's people, shared group, slip, and picks
// through the same services a person uses: auth.SignIn, the invite join, the
// slip store, and slips.Service.AddLeg. It must run after the morning ingest
// has produced the week and before the first app view.
func SeedScenario(
	ctx context.Context,
	store *db.Store,
	service *slips.Service,
	week db.Week,
) (Seed, error) {
	if store == nil {
		return Seed{}, errors.New("seed sandbox: no store")
	}
	if service == nil {
		return Seed{}, errors.New("seed sandbox: no slip service")
	}

	people := make([]SeededPerson, 0, len(Personas()))
	users := make(map[PersonaID]db.User, len(Personas()))

	var shared db.Group
	for i, persona := range SharedPersonas() {
		user, err := auth.SignIn(ctx, store, auth.Identity{
			Sub:   "dev:" + persona.Email,
			Email: persona.Email,
			Name:  persona.Name,
		})
		if err != nil {
			return Seed{}, fmt.Errorf("seed persona %s: %w", persona.ID, err)
		}
		users[persona.ID] = user

		if i == 0 {
			shared, err = store.Create(ctx, user.ID, SharedGroupName)
			if err != nil {
				return Seed{}, fmt.Errorf("seed shared group: %w", err)
			}
		} else if _, err := store.JoinByToken(ctx, user.ID, shared.InviteToken); err != nil {
			return Seed{}, fmt.Errorf("join persona %s to shared group: %w", persona.ID, err)
		}
		people = append(people, SeededPerson{Persona: persona, User: user, GroupID: shared.ID})
	}

	outsider := Outsider()
	outsiderUser, err := auth.SignIn(ctx, store, auth.Identity{
		Sub:   "dev:" + outsider.Email,
		Email: outsider.Email,
		Name:  outsider.Name,
	})
	if err != nil {
		return Seed{}, fmt.Errorf("seed persona %s: %w", outsider.ID, err)
	}
	users[outsider.ID] = outsiderUser
	outsiderGroup, err := onlyGroup(ctx, store, outsider, outsiderUser)
	if err != nil {
		return Seed{}, err
	}
	people = append(people, SeededPerson{
		Persona: outsider,
		User:    outsiderUser,
		GroupID: outsiderGroup.ID,
	})

	sharedIDs := make([]int64, 0, len(SharedPersonas()))
	for _, person := range people {
		if person.Persona.Shared {
			sharedIDs = append(sharedIDs, person.User.ID)
		}
	}
	slip, err := store.CreateSlip(ctx, shared.ID, week.ID, slipName, legsPerMember, sharedIDs)
	if err != nil {
		return Seed{}, fmt.Errorf("seed slip: %w", err)
	}

	legs := make(map[LegID]int64, len(LegDefinitions()))
	for _, definition := range LegDefinitions() {
		owner, ok := users[definition.Persona]
		if !ok {
			return Seed{}, fmt.Errorf("seed leg %s: unknown persona %s", definition.ID, definition.Persona)
		}
		created, err := service.AddLeg(ctx, owner.ID, db.Leg{
			SlipID:     slip.ID,
			GameID:     new(definition.GameID),
			MarketType: definition.MarketType,
			Selection:  definition.Selection,
			Team:       definition.Team,
			Side:       definition.Side,
			Line:       definition.Line,
			Odds:       definition.Odds,
		})
		if err != nil {
			return Seed{}, fmt.Errorf("seed leg %s: %w", definition.ID, err)
		}
		legs[definition.ID] = created.ID
	}

	return Seed{
		Week:          week,
		SharedGroup:   shared,
		OutsiderGroup: outsiderGroup,
		Slip:          slip,
		People:        people,
		Legs:          legs,
	}, nil
}

// onlyGroup returns the single group auth.SignIn created for persona. A fresh
// sign-in always makes one; anything else means the seed is running against a
// database that was not cleared first.
func onlyGroup(
	ctx context.Context,
	store *db.Store,
	persona Persona,
	user db.User,
) (db.Group, error) {
	groups, err := store.ForUser(ctx, user.ID)
	if err != nil {
		return db.Group{}, fmt.Errorf("seed persona %s groups: %w", persona.ID, err)
	}
	if len(groups) != 1 {
		return db.Group{}, fmt.Errorf(
			"seed persona %s: sign-in made %d groups, want 1", persona.ID, len(groups))
	}
	return groups[0], nil
}
