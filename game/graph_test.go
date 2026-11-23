package game

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The dome graph is a table typed by hand in InitSegments: 49 segments with
// eight hops each, 392 links. The tests in this file look at every one of
// them, and at every one of the 98 segment ends and of the 98 ways an eel
// can be travelling (49 segments, two directions), so what they say holds
// for the whole table and not for a sample of it.
//
// They show that the table is well formed, that it agrees with itself, and
// that it is still the table written down in testdata/dome-graph.txt. They
// cannot show that the table matches the dome: which pipe really meets
// which, and which way is up or left for someone standing at the joystick,
// can only be checked at the dome, or by reading the listing against
// docs/lights-diagram.png.

// The four turns a hop table has a slot for.
const (
	turnUp = iota
	turnDown
	turnLeft
	turnRight
	turnCount
)

var turnNames = [turnCount]string{"up", "down", "left", "right"}

// hopSlots names the eight hop fields of a segment, in the order
// InitSegments fills them in: the four turns for an eel leaving by the
// high end, then the four for one leaving by the low end.
var hopSlots = [2 * turnCount]struct {
	name   string
	travel Direction
	turn   int
	hop    func(*Segment) *Hop
}{
	{"GreaterUp", Greater, turnUp, func(s *Segment) *Hop { return s.GreaterUp }},
	{"GreaterDown", Greater, turnDown, func(s *Segment) *Hop { return s.GreaterDown }},
	{"GreaterLeft", Greater, turnLeft, func(s *Segment) *Hop { return s.GreaterLeft }},
	{"GreaterRight", Greater, turnRight, func(s *Segment) *Hop { return s.GreaterRight }},
	{"LesserUp", Lesser, turnUp, func(s *Segment) *Hop { return s.LesserUp }},
	{"LesserDown", Lesser, turnDown, func(s *Segment) *Hop { return s.LesserDown }},
	{"LesserLeft", Lesser, turnLeft, func(s *Segment) *Hop { return s.LesserLeft }},
	{"LesserRight", Lesser, turnRight, func(s *Segment) *Hop { return s.LesserRight }},
}

// turnHop returns the hop a segment has for an eel travelling in the given
// direction that takes the given turn.
func turnHop(s *Segment, travel Direction, turn int) *Hop {
	if travel == Lesser {
		turn += turnCount
	}
	return hopSlots[turn].hop(s)
}

// domeEnd is one end of one segment. End 2*i is the low end of segment i,
// position 0, and end 2*i+1 is its high end, position 21. An eel
// travelling towards the high end leaves its segment there, by the Greater
// hops; a hop that comes onto a segment at its high end sets the eel
// travelling towards the low end.
type domeEnd int

func endOf(segment int, high bool) domeEnd {
	if high {
		return domeEnd(2*segment + 1)
	}
	return domeEnd(2 * segment)
}

func (e domeEnd) segment() int { return int(e) / 2 }
func (e domeEnd) high() bool   { return e%2 == 1 }

// far returns the other end of the same segment.
func (e domeEnd) far() domeEnd { return e ^ 1 }

// leaving returns the direction an eel travels in to leave by this end.
func (e domeEnd) leaving() Direction {
	if e.high() {
		return Greater
	}
	return Lesser
}

// dome is the hop table as these tests read it.
type dome struct {
	segments []*Segment
	// leads[e][turn] is the end at which an eel comes onto its next
	// segment when it leaves by end e with the stick at that turn.
	leads [][turnCount]domeEnd
}

// name returns the name InitSegments gives the hop that comes onto a
// segment at this end: p2Start for the low end of p2, r9End for the high
// end of r9.
func (d *dome) name(e domeEnd) string {
	if e.high() {
		return d.segments[e.segment()].Label + "End"
	}
	return d.segments[e.segment()].Label + "Start"
}

func (d *dome) names(ends []domeEnd) []string {
	names := make([]string, len(ends))
	for i, e := range ends {
		names[i] = d.name(e)
	}
	return names
}

// named returns the end with the given name.
func (d *dome) named(t *testing.T, name string) domeEnd {
	t.Helper()
	for e := range d.leads {
		if d.name(domeEnd(e)) == name {
			return domeEnd(e)
		}
	}
	t.Fatalf("the graph has no segment end called %s", name)
	return 0
}

// neighbours returns the ends that e leads to, each once, lowest first.
func (d *dome) neighbours(e domeEnd) []domeEnd {
	var ends []domeEnd
	for _, to := range d.leads[e] {
		if !slices.Contains(ends, to) {
			ends = append(ends, to)
		}
	}
	slices.Sort(ends)
	return ends
}

// junctions returns the segment ends in groups: two ends are in the same
// group if one leads to the other, or if a chain of such links joins them.
// Each group has its lowest end first, and the groups are in that order.
func (d *dome) junctions() [][]domeEnd {
	linked := make([][]domeEnd, len(d.leads))
	for e := range d.leads {
		for _, to := range d.leads[e] {
			linked[e] = append(linked[e], to)
			linked[to] = append(linked[to], domeEnd(e))
		}
	}
	var groups [][]domeEnd
	grouped := make([]bool, len(d.leads))
	for e := range d.leads {
		if grouped[e] {
			continue
		}
		grouped[e] = true
		group := []domeEnd{domeEnd(e)}
		for i := 0; i < len(group); i++ {
			for _, to := range linked[group[i]] {
				if !grouped[to] {
					grouped[to] = true
					group = append(group, to)
				}
			}
		}
		slices.Sort(group)
		groups = append(groups, group)
	}
	return groups
}

// hopName names where a hop leads as InitSegments does, p2Start or r9End,
// and says what is there in plain words if it is not a hop InitSegments
// could have named.
func hopName(h *Hop) string {
	switch {
	case h == nil || h.Point == nil || h.Point.Segment == nil:
		return "nothing"
	case h.Point.Position == 0 && h.Direction == Greater:
		return h.Point.Segment.Label + "Start"
	case h.Point.Position == testSegmentLength-1 && h.Direction == Lesser:
		return h.Point.Segment.Label + "End"
	}
	return fmt.Sprintf("%s position %d %s", h.Point.Segment.Label, h.Point.Position, towards(h.Direction))
}

// readDome reads the table InitSegments builds, and returns with it
// everything that keeps a link from being a link: a slot left empty, a hop
// that does not come onto a segment of the graph at one of its two ends
// travelling inwards, or one that leads back onto the segment it leaves.
func readDome() (*dome, []string) {
	segments := InitSegments()
	d := &dome{segments: segments[:], leads: make([][turnCount]domeEnd, 2*len(segments))}
	index := map[*Segment]int{}
	for i, s := range segments {
		if s == nil {
			return d, []string{fmt.Sprintf("segment %d of the graph is missing", i)}
		}
		index[s] = i
	}

	var problems []string
	for i, s := range segments {
		for _, slot := range hopSlots {
			where := s.Label + "." + slot.name
			h := slot.hop(s)
			if h == nil || h.Point == nil || h.Point.Segment == nil {
				problems = append(problems, where+" is not filled in")
				continue
			}
			from := endOf(i, slot.travel == Greater)
			to, inGraph := index[h.Point.Segment]
			switch {
			case !inGraph:
				problems = append(problems, fmt.Sprintf("%s leads onto a segment labelled %q that is not one of the graph's own", where, h.Point.Segment.Label))
			case to == i:
				problems = append(problems, fmt.Sprintf("%s leads back onto %s itself", where, s.Label))
			case h.Point.Position == 0 && h.Direction == Greater:
				d.leads[from][slot.turn] = endOf(to, false)
			case h.Point.Position == testSegmentLength-1 && h.Direction == Lesser:
				d.leads[from][slot.turn] = endOf(to, true)
			default:
				problems = append(problems, fmt.Sprintf(
					"%s comes onto %s at position %d %s: a hop has to come on at position 0 travelling towards the high end, or at position %d travelling towards the low end",
					where, h.Point.Segment.Label, h.Point.Position, towards(h.Direction), testSegmentLength-1))
			}
		}
	}
	if len(problems) > 0 {
		return d, problems
	}

	// NextHop decides a diagonal by comparing two hops as pointers, so two
	// turns of one junction that lead to the same place have to hold the
	// same Hop, not two that say the same thing.
	for e := range d.leads {
		s, travel := segments[domeEnd(e).segment()], domeEnd(e).leaving()
		for a := range turnCount {
			for b := a + 1; b < turnCount; b++ {
				if d.leads[e][a] == d.leads[e][b] && turnHop(s, travel, a) != turnHop(s, travel, b) {
					problems = append(problems, fmt.Sprintf(
						"leaving %s %s, %s and %s both lead to %s but through two different Hops, which NextHop takes for two different ways",
						s.Label, towards(travel), turnNames[a], turnNames[b], d.name(d.leads[e][a])))
				}
			}
		}
	}
	return d, problems
}

// fewErrors reports the first failures of a test that looks at hundreds of
// cases and counts the rest, so that one mistake which shows in every case
// does not bury what it is.
type fewErrors struct {
	t    *testing.T
	more int
	left int
}

func reportFew(t *testing.T) *fewErrors {
	f := &fewErrors{t: t, left: 10}
	t.Cleanup(func() {
		if f.more > 0 {
			t.Errorf("and %d more failures like the last ones", f.more)
		}
	})
	return f
}

func (f *fewErrors) Errorf(format string, args ...any) {
	f.t.Helper()
	if f.left == 0 {
		f.more++
		return
	}
	f.left--
	f.t.Errorf(format, args...)
}

// wellFormedDome is readDome for the tests that need every link to be one.
func wellFormedDome(t *testing.T) *dome {
	t.Helper()
	d, problems := readDome()
	if len(problems) > 0 {
		t.Fatalf("the hop table is not well formed, so this cannot be checked: %s (%d such in all; TestDomeHopsEnterAtSegmentEnds lists them)",
			problems[0], len(problems))
	}
	return d
}

// TestDomeSegmentsCoverEveryPixelOnce: the graph is 49 segments of 22
// pixels, and segment i owns pixels 22*i to 22*i+21 of the frame, so that
// each of the frame's 1078 pixels belongs to exactly one segment.
func TestDomeSegmentsCoverEveryPixelOnce(t *testing.T) {
	if SegmentCount != 49 || SegmentLength != 22 || SegmentCount*SegmentLength != testPixels {
		t.Errorf("the dome is %d segments of %d pixels, %d pixels in all, want 49 of 22 and 1078",
			SegmentCount, SegmentLength, SegmentCount*SegmentLength)
	}

	// The five sections in the order InitSegments numbers them. The blue
	// one, with the door in it, is a segment short.
	var labels []string
	for _, section := range []struct {
		letter   string
		segments int
	}{{"p", 10}, {"r", 10}, {"b", 9}, {"g", 10}, {"y", 10}} {
		for n := 1; n <= section.segments; n++ {
			labels = append(labels, fmt.Sprintf("%s%d", section.letter, n))
		}
	}

	segments := InitSegments()
	if len(segments) != testSegments || len(labels) != testSegments {
		t.Fatalf("the graph has %d segments and the test names %d, want 49 of each", len(segments), len(labels))
	}
	var owners [testPixels]int
	seen := map[*Segment]int{}
	for i, s := range segments {
		if s == nil {
			t.Fatalf("segment %d of the graph is missing", i)
		}
		if first, twice := seen[s]; twice {
			t.Errorf("segments %d and %d of the graph are one and the same Segment", first, i)
		}
		seen[s] = i
		if s.Label != labels[i] {
			t.Errorf("segment %d is labelled %q, want %q", i, s.Label, labels[i])
		}
		if s.Offset != i*testSegmentLength {
			t.Errorf("segment %d (%s) starts at pixel %d, want %d", i, s.Label, s.Offset, i*testSegmentLength)
			continue
		}
		if s.Length != testSegmentLength {
			t.Errorf("segment %d (%s) says it is %d pixels long, want 22", i, s.Label, s.Length)
		}
		for p := range testSegmentLength {
			owners[s.Offset+p]++
		}
	}
	few := reportFew(t)
	for px, n := range owners {
		if n != 1 {
			few.Errorf("pixel %d belongs to %d segments, want exactly one", px, n)
		}
	}
}

// TestDomeHopsEnterAtSegmentEnds: every one of the 392 slots of the table
// is filled, with a hop that comes onto another segment of the graph at
// one of its ends and travelling away from that end: position 0 towards
// the high end, or position 21 towards the low end.
func TestDomeHopsEnterAtSegmentEnds(t *testing.T) {
	d, problems := readDome()
	few := reportFew(t)
	for _, problem := range problems {
		few.Errorf("%s", problem)
	}
	if slots := len(d.segments) * len(hopSlots); slots != 392 {
		t.Errorf("the table has %d slots, want 49 x 8 = 392", slots)
	}
}

// TestDomeLinksLeadBothWays: if an eel can leave by one segment end and
// come on at another, then an eel leaving by that other end can come on at
// the first. A link typed with the wrong segment or the wrong end nearly
// always breaks this, because the segment it names does not lead back.
func TestDomeLinksLeadBothWays(t *testing.T) {
	d := wellFormedDome(t)
	few := reportFew(t)
	for e := range d.leads {
		from := domeEnd(e)
		for turn, to := range d.leads[from] {
			if !slices.Contains(d.leads[to][:], from) {
				few.Errorf("%s leads to %s with the stick %s, but nothing leads from %s back to %s",
					d.name(from), d.name(to), turnNames[turn], d.name(to), d.name(from))
			}
		}
	}
}

// TestDomeJunctionShapes counts the junctions of the table and pins what
// each looks like. There are three kinds:
//
//   - 28 junctions where three segment ends meet, and each end leads to the
//     other two;
//   - 2 joints in the blue section, the one with the door, where two
//     segments meet end to end with nothing else there: b2 with b3, and
//     b6 with b7;
//   - the ten ends at the foot of the dome: the low end of the first
//     segment of each section and the high end of its last. In
//     docs/lights-diagram.png they are the ten loose ends round the
//     outside, which meet no other segment. The table joins them into a
//     ring: each leads to the loose end on either side of it and to no
//     other.
//
// That is 28*3 + 2*2 + 10 = 98 ends, all there are. The ring is the one
// place where linked ends do not all lead to each other. The test pins
// that as it is, so that nobody takes it for links left out.
func TestDomeJunctionShapes(t *testing.T) {
	d := wellFormedDome(t)

	// leadsToAllOthers says whether every end of the group leads to
	// exactly the other ends of the group.
	leadsToAllOthers := func(group []domeEnd) bool {
		for _, e := range group {
			others := slices.DeleteFunc(slices.Clone(group), func(o domeEnd) bool { return o == e })
			if !slices.Equal(d.neighbours(e), others) {
				return false
			}
		}
		return true
	}
	// isRing says whether every end of the group leads to exactly two
	// others of the group. A group is all in one piece, so where links
	// lead both ways that makes a group of more than three one closed
	// ring.
	isRing := func(group []domeEnd) bool {
		for _, e := range group {
			near := d.neighbours(e)
			if len(near) != 2 || !slices.Contains(group, near[0]) || !slices.Contains(group, near[1]) {
				return false
			}
		}
		return len(group) > 3
	}

	threeWay, ringEnds := 0, 0
	var twoWay, rings []string
	for _, group := range d.junctions() {
		names := strings.Join(d.names(group), " ")
		switch {
		case len(group) == 3 && leadsToAllOthers(group):
			threeWay++
		case len(group) == 2 && leadsToAllOthers(group):
			twoWay = append(twoWay, names)
		case isRing(group):
			rings = append(rings, names)
			ringEnds += len(group)
		default:
			t.Errorf("the ends %s are linked, but not as a three-way junction, a two-way joint or a ring", names)
		}
	}
	if threeWay != 28 {
		t.Errorf("the table has %d three-way junctions, want 28", threeWay)
	}
	if want := []string{"b2End b3Start", "b6End b7Start"}; !slices.Equal(twoWay, want) {
		t.Errorf("the two-way joints of the table are %q, want %q", twoWay, want)
	}
	if len(rings) != 1 || ringEnds != 10 {
		t.Errorf("the table has %d rings, of %d ends in all (%q), want the one ring of 10 ends at the foot of the dome", len(rings), ringEnds, rings)
	}

	// The ends of the ring in order. It closes from the last back to the
	// first.
	ring := []string{
		"p1Start", "p10End", "y1Start", "y10End", "g1Start",
		"g10End", "b1Start", "b9End", "r1Start", "r10End",
	}
	for i, name := range ring {
		before, after := ring[(i+len(ring)-1)%len(ring)], ring[(i+1)%len(ring)]
		got := d.neighbours(d.named(t, name))
		want := []domeEnd{d.named(t, before), d.named(t, after)}
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s leads to %q, want the two ends beside it in the ring, %q", name, d.names(got), d.names(want))
		}
	}
}

// TestDomeLeftAndRightAreUpOrDown: a junction has at most two ways on, and
// the table gives each of them twice, once on each axis of the stick. So
// left and right always lead where up or down leads, and where up and down
// lead two ways, left and right lead those same two ways, one each. That
// is the case at 94 of the 98 segment ends; at the other four, the two-way
// joints, all four turns lead the one way there is.
//
// Which of the two is left and which is right is not something a rule can
// tell: only the listing pins that, and only the dome can say if it is
// true.
func TestDomeLeftAndRightAreUpOrDown(t *testing.T) {
	d := wellFormedDome(t)
	few := reportFew(t)
	forks := 0
	for e := range d.leads {
		leads := d.leads[e]
		up, down, left, right := leads[turnUp], leads[turnDown], leads[turnLeft], leads[turnRight]
		leaving := fmt.Sprintf("leaving %s %s", d.segments[domeEnd(e).segment()].Label, towards(domeEnd(e).leaving()))
		for _, turn := range []int{turnLeft, turnRight} {
			if leads[turn] != up && leads[turn] != down {
				few.Errorf("%s, %s leads to %s, where neither up (%s) nor down (%s) leads",
					leaving, turnNames[turn], d.name(leads[turn]), d.name(up), d.name(down))
			}
		}
		if up != down {
			forks++
			if left == right {
				few.Errorf("%s, up leads to %s and down to %s, but left and right both lead to %s",
					leaving, d.name(up), d.name(down), d.name(left))
			}
		}
	}
	if forks != 94 {
		t.Errorf("up and down lead two ways at %d of the %d segment ends, want 94 of 98", forks, len(d.leads))
	}
}

// TestDomeEveryStateReachesEveryOther: an eel is on one of 49 segments,
// travelling one of two ways, and from each of those 98 states the turns
// of the table can bring it to all 98. No part of the dome is cut off, and
// none can be entered but not left, or ridden in one direction only.
func TestDomeEveryStateReachesEveryOther(t *testing.T) {
	d := wellFormedDome(t)
	// A state is named by the end the eel is travelling towards. The turn
	// it takes there brings it onto another segment at one end, travelling
	// towards the far one.
	states := len(d.leads)
	if states != 98 {
		t.Errorf("the graph has %d states, want 49 x 2 = 98", states)
	}
	few := reportFew(t)
	describe := func(e domeEnd) string {
		return fmt.Sprintf("on %s %s", d.segments[e.segment()].Label, towards(e.leaving()))
	}
	for start := range states {
		reached := make([]bool, states)
		reached[start] = true
		queue := []domeEnd{domeEnd(start)}
		for i := 0; i < len(queue); i++ {
			for _, onto := range d.leads[queue[i]] {
				if next := onto.far(); !reached[next] {
					reached[next] = true
					queue = append(queue, next)
				}
			}
		}
		for state, ok := range reached {
			if !ok {
				few.Errorf("an eel %s can never come to be %s", describe(domeEnd(start)), describe(domeEnd(state)))
			}
		}
	}
}

var updateDomeListing = flag.Bool("update-dome-graph", false,
	"write game/testdata/dome-graph.txt again from the table in InitSegments")

// domeListingHeader is the head of testdata/dome-graph.txt.
const domeListingHeader = `# The dome graph: where each of the 392 links of the table in game/init.go
# leads, in the order of the table. "p1.GreaterUp = p2Start" says that an
# eel leaving p1 by its high end (Greater, position 21) with the stick up
# comes onto p2 at its low end (Start, position 0). Lesser is leaving by
# the low end, and End is coming on at the high end.
#
# TestDomeGraphMatchesItsListing compares this file with the table, so that
# no link changes by accident. After changing the table on purpose, write
# this file again with
#
#   dev/run go test ./game/ -run TestDomeGraphMatchesItsListing -update-dome-graph
#
# and read what changed here when reviewing: it is every link that now
# leads somewhere else. Nothing in this file shows that the links match the
# dome itself. That can only be checked at the dome, or against
# docs/lights-diagram.png.
`

// domeListing writes the table out, one line for each link.
func domeListing(segments []*Segment) string {
	var b strings.Builder
	b.WriteString(domeListingHeader)
	for _, s := range segments {
		b.WriteString("\n")
		for _, slot := range hopSlots {
			fmt.Fprintf(&b, "%s.%s = %s\n", s.Label, slot.name, hopName(slot.hop(s)))
		}
	}
	return b.String()
}

// TestDomeGraphMatchesItsListing compares the table with a listing of it
// kept in testdata/dome-graph.txt, link for link.
//
// The rules the other tests check cannot see every wrong link. A hop to the
// wrong one of two segments that both meet the junction, or left and right
// exchanged, leaves the table as well formed as it was. Such a slip is
// only visible as a change, so the table is written down a second time,
// and an edit of init.go that was not meant shows up here as a difference.
// An edit that was meant has to be made in both places, and the difference
// in the listing is the part a reviewer should read: one line for each
// link that now leads somewhere else.
//
// The listing was written from the table, so it is no more right about the
// dome than the table is. It says what the table was, not what the dome
// is.
func TestDomeGraphMatchesItsListing(t *testing.T) {
	segments := InitSegments()
	for i, s := range segments {
		if s == nil {
			t.Fatalf("segment %d of the graph is missing", i)
		}
	}
	got := domeListing(segments[:])
	path := filepath.Join("testdata", "dome-graph.txt")
	if *updateDomeListing {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing the listing: %v", err)
		}
		t.Logf("wrote game/%s from the table in InitSegments", filepath.ToSlash(path))
		return
	}

	const regenerate = "dev/run go test ./game/ -run TestDomeGraphMatchesItsListing -update-dome-graph"
	listing, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the listing of the dome graph: %v\nIt can be written from the table with\n  %s", err, regenerate)
	}
	if got == string(listing) {
		return
	}

	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(listing), "\n")
	lineOf := func(lines []string, i int) string {
		if i < len(lines) {
			return lines[i]
		}
		return "(no such line)"
	}
	few := reportFew(t)
	differ := 0
	for i := range max(len(gotLines), len(wantLines)) {
		g, w := lineOf(gotLines, i), lineOf(wantLines, i)
		if g == w {
			continue
		}
		differ++
		few.Errorf("line %d: the table in init.go says %q, the listing says %q", i+1, g, w)
	}
	t.Errorf("the table in game/init.go and its listing game/testdata/dome-graph.txt differ, in %d of their lines.\n"+
		"If the table was not meant to change, the lines above name the links to put back.\n"+
		"If it was, write the listing again with\n  %s\n"+
		"and review the change to the listing: it is the list of links that now lead somewhere else.",
		differ, regenerate)
}
