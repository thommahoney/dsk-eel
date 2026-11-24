package game

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thommahoney/dsk-eel/controller"
)

// This file holds a second model of one tick of the game, written from the
// rules and not from Eel.Move, and the checker that plays a real game and
// the model side by side and compares them after every tick: the eel point
// for point, what it has left to grow, the way it travels, the food and
// whether it is fresh, every pixel of the frame, the map of the pixels the
// eel is on, and the bytes of the seven Art-Net packets.
//
// The model shares no code with the tick. It does not call NextHop, Chomp,
// drawBody, hsvToRGB, Draw or the food placement, and it does not read the
// table in InitSegments: its dome is read from the listing of the links in
// testdata/dome-graph.txt. It keeps the eel as pixel numbers, tail first,
// where the game keeps Points, head first. Sizes and the wire layout are
// spelled out below in the model's own literals, so that a wrong constant
// in the game shows as a difference instead of being followed.
//
// What that buys is that a mistake has to be made twice, in two different
// shapes, to go unseen. It does not make the model right: a rule that is
// misunderstood the same way in both places still passes. And the game
// makes two choices by chance that a test cannot make for it, the turn
// demo mode takes and where new food goes. There the model says which
// outcomes the rules allow, checks that the game's is one of them, and
// goes on from the one the game took.

// The dome and the game as the model knows them.
const (
	tickModelSegments      = 49
	tickModelSegmentLength = 22
	tickModelPixels        = 1078 // 49 x 22
	tickModelEnds          = 98   // every segment has a low end and a high end
	tickModelLinks         = 392  // four turns at each end
	tickModelFoodLength    = 7    // of a food, of a new eel, and what a meal adds
	tickModelPlacements    = 784  // a food's first point can be at positions 6 to 21 of 49 segments
	tickModelSignalLength  = 5

	tickModelUniverses    = 7
	tickModelFullUniverse = 170 // pixels in each universe but the last, which has the other 58
	tickModelHeaderLen    = 18
	tickModelPacketLen    = 530 // the header and 512 bytes of data
)

// The four turns a junction has a link for, in the order the model keeps
// them.
const (
	tickModelUp = iota
	tickModelDown
	tickModelLeft
	tickModelRight
	tickModelTurns
)

// tickModelStickTurns is the steering rule as the model states it: the
// turns each position of the stick asks for. Straight along an axis asks
// for one, a diagonal for two, and the centred stick for none, like the
// seven values of the four stick bits that are no position at all.
var tickModelStickTurns = map[controller.Direction][]int{
	controller.Dir_North:     {tickModelUp},
	controller.Dir_South:     {tickModelDown},
	controller.Dir_West:      {tickModelLeft},
	controller.Dir_East:      {tickModelRight},
	controller.Dir_NorthEast: {tickModelUp, tickModelRight},
	controller.Dir_SouthEast: {tickModelDown, tickModelRight},
	controller.Dir_SouthWest: {tickModelDown, tickModelLeft},
	controller.Dir_NorthWest: {tickModelUp, tickModelLeft},
}

// tickModelStickValues is how many values the four bits of the stick have.
const tickModelStickValues = 16

// tickModelCardinals are the four stick positions that ask for one turn,
// in the order of the turns.
var tickModelCardinals = [tickModelTurns]controller.Direction{
	controller.Dir_North, controller.Dir_South, controller.Dir_West, controller.Dir_East,
}

// tickModelDome is the dome as the listing in testdata/dome-graph.txt
// gives it. A segment end is a number: 2*s is the low end of segment s,
// position 0, and 2*s+1 its high end, position 21.
type tickModelDome struct {
	labels [tickModelSegments]string

	// leads[end][turn] is the end at which an eel comes onto its next
	// segment when it leaves by end with the stick at that turn.
	leads [tickModelEnds][tickModelTurns]int

	// junctionsTo[end][s] is the fewest junctions an eel travelling
	// towards end has to cross before it is on segment s.
	junctionsTo [tickModelEnds][tickModelSegments]int
}

// tickModelReadDome reads the listing. The labels are the five sections in
// the order the frame has them, purple, red, blue, green and yellow, with
// ten segments each but for blue, which has nine.
func tickModelReadDome(t testing.TB) *tickModelDome {
	t.Helper()
	d := &tickModelDome{}
	number := map[string]int{}
	for _, section := range []struct {
		letter   string
		segments int
	}{{"p", 10}, {"r", 10}, {"b", 9}, {"g", 10}, {"y", 10}} {
		for n := 1; n <= section.segments; n++ {
			label := fmt.Sprintf("%s%d", section.letter, n)
			d.labels[len(number)] = label
			number[label] = len(number)
		}
	}

	// Each of the eight fields of a segment: whether it is for leaving by
	// the high end, and for which turn.
	slots := map[string]struct {
		high bool
		turn int
	}{
		"GreaterUp": {true, tickModelUp}, "GreaterDown": {true, tickModelDown},
		"GreaterLeft": {true, tickModelLeft}, "GreaterRight": {true, tickModelRight},
		"LesserUp": {false, tickModelUp}, "LesserDown": {false, tickModelDown},
		"LesserLeft": {false, tickModelLeft}, "LesserRight": {false, tickModelRight},
	}

	path := filepath.Join("testdata", "dome-graph.txt")
	listing, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the listing of the dome graph: %v", err)
	}
	var listed [tickModelEnds][tickModelTurns]bool
	links := 0
	for n, line := range strings.Split(string(listing), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		bad := func(why string) {
			t.Helper()
			t.Fatalf("game/%s line %d (%q): %s", filepath.ToSlash(path), n+1, line, why)
		}
		from, to, ok := strings.Cut(line, " = ")
		if !ok {
			bad("want a link like p1.GreaterUp = p2Start")
		}
		label, field, _ := strings.Cut(from, ".")
		segment, known := number[label]
		slot, isSlot := slots[field]
		if !known || !isSlot {
			bad("the left side names no hop of a segment of the dome")
		}
		var onto int
		if l, start := strings.CutSuffix(to, "Start"); start {
			onto = 2 * number[l]
			label = l
		} else if l, end := strings.CutSuffix(to, "End"); end {
			onto = 2*number[l] + 1
			label = l
		} else {
			bad("the right side is neither the Start nor the End of a segment")
		}
		if _, known := number[label]; !known {
			bad("the right side names no segment of the dome")
		}
		end := 2 * segment
		if slot.high {
			end++
		}
		if listed[end][slot.turn] {
			bad("this link is listed twice")
		}
		listed[end][slot.turn] = true
		d.leads[end][slot.turn] = onto
		links++
	}
	if links != tickModelLinks {
		t.Fatalf("game/%s lists %d links, want 49 x 8 = 392", filepath.ToSlash(path), links)
	}

	// How far every segment is from every way of travelling, by going
	// outwards one junction at a time. An eel that has come on at an end
	// travels towards the other end of that segment, which is end^1.
	for from := range tickModelEnds {
		reach := &d.junctionsTo[from]
		for s := range reach {
			reach[s] = -1
		}
		reach[from/2] = 0
		queue := []int{from}
		for i := 0; i < len(queue); i++ {
			for _, onto := range d.leads[queue[i]] {
				if reach[onto/2] < 0 {
					reach[onto/2] = reach[queue[i]/2] + 1
					queue = append(queue, onto^1)
				}
			}
		}
	}
	return d
}

// steer is the steering rule: the end the stick leads onto for an eel that
// leaves by end, and whether it leads anywhere. One turn leads where its
// link does. Two lead on only if both their links come onto the same end,
// which is the same place travelling the same way.
func (d *tickModelDome) steer(end int, stick controller.Direction) (onto int, ok bool) {
	turns := tickModelStickTurns[stick]
	if len(turns) == 0 {
		return 0, false
	}
	onto = d.leads[end][turns[0]]
	for _, turn := range turns[1:] {
		if d.leads[end][turn] != onto {
			return 0, false
		}
	}
	return onto, true
}

// where names a pixel for a message: segment and position.
func (d *tickModelDome) where(pixel int) string {
	if pixel < 0 || pixel >= tickModelPixels {
		return fmt.Sprintf("pixel %d, which the dome does not have", pixel)
	}
	return fmt.Sprintf("%s position %d", d.labels[pixel/tickModelSegmentLength], pixel%tickModelSegmentLength)
}

// tickModelInput is what a tick is played by: the stick, whether the game
// is a demo, whether the joystick is lost, and how bright the dome is.
type tickModelInput struct {
	stick      controller.Direction
	demo, lost bool
	brightness float64
}

func (in tickModelInput) String() string {
	s := fmt.Sprintf("stick %v (%04b)", in.stick, int(in.stick))
	if in.demo {
		s += ", demo"
	}
	if in.lost {
		s += ", joystick lost"
	}
	return fmt.Sprintf("%s, brightness %v", s, in.brightness)
}

// tickModelState is a game as the model keeps it.
type tickModelState struct {
	// trail is the pixels the eel is on, the tail first and the head
	// last. Pixel 22*s+p is position p of segment s.
	trail  []int
	growth int
	// high says the eel travels towards the high end of the segment its
	// head is on.
	high bool

	// The food is on positions foodLow to foodHigh of foodSegment, and is
	// all eaten when foodLow is above foodHigh. It is fresh until its
	// first bite.
	foodSegment, foodLow, foodHigh int
	fresh                          bool
}

func (m *tickModelState) head() int { return m.trail[len(m.trail)-1] }

// clone returns a copy that shares nothing with m.
func (m *tickModelState) clone() tickModelState {
	c := *m
	c.trail = append([]int(nil), m.trail...)
	return c
}

func (m *tickModelState) describe(d *tickModelDome) string {
	if len(m.trail) == 0 {
		return "a game with no eel yet"
	}
	var b strings.Builder
	towards := "low"
	if m.high {
		towards = "high"
	}
	fmt.Fprintf(&b, "an eel of %d with its head on %s travelling towards the %s end and %d to grow; ",
		len(m.trail), d.where(m.head()), towards, m.growth)
	if m.foodLow > m.foodHigh {
		b.WriteString("the food all eaten; ")
	} else {
		fmt.Fprintf(&b, "food on %s positions %d to %d (fresh=%v); ", d.labels[m.foodSegment], m.foodLow, m.foodHigh, m.fresh)
	}
	b.WriteString("eel pixels, head first:")
	for i := len(m.trail) - 1; i >= 0; i-- {
		fmt.Fprintf(&b, " %d", m.trail[i])
	}
	return b.String()
}

// leaving returns the end the eel is travelling towards, and whether its
// head is on that end's pixel, so that the next tick takes it through the
// junction there.
func (m *tickModelState) leaving() (end int, now bool) {
	segment, position := m.head()/tickModelSegmentLength, m.head()%tickModelSegmentLength
	if m.high {
		return 2*segment + 1, position == tickModelSegmentLength-1
	}
	return 2 * segment, position == 0
}

// tickModelWay is one place the head can go on a tick.
type tickModelWay struct {
	pixel int
	high  bool // the way the eel travels from there
	onto  int  // the end it comes on at, or -1 on a tick along a segment
}

// tickModelArriving is the way onto a segment at the given end: the eel is
// on that end's pixel and travels towards the other end.
func tickModelArriving(onto int) tickModelWay {
	pixel := onto / 2 * tickModelSegmentLength
	if onto%2 == 1 {
		pixel += tickModelSegmentLength - 1
	}
	return tickModelWay{pixel: pixel, high: onto%2 == 0, onto: onto}
}

// ways returns where the rules let the head go on this tick. Along a
// segment that is the next pixel. At the end of one it is where the stick
// leads; with the stick centred in demo mode, or with the joystick lost,
// it is where any of the four turns leads, since the game picks one of
// them by chance; and it is nowhere if the stick leads nowhere, which ends
// the game.
func (m *tickModelState) ways(d *tickModelDome, in tickModelInput) (ways [tickModelTurns]tickModelWay, n int) {
	end, now := m.leaving()
	if !now {
		step := -1
		if m.high {
			step = 1
		}
		ways[0] = tickModelWay{pixel: m.head() + step, high: m.high, onto: -1}
		return ways, 1
	}
	if in.stick == controller.Dir_Neutral && (in.demo || in.lost) {
	turns:
		for _, onto := range d.leads[end] {
			for _, w := range ways[:n] {
				if w.onto == onto {
					continue turns
				}
			}
			ways[n] = tickModelArriving(onto)
			n++
		}
		return ways, n
	}
	if onto, ok := d.steer(end, in.stick); ok {
		ways[0] = tickModelArriving(onto)
		return ways, 1
	}
	return ways, 0
}

// advance moves the head to w. An eel with something left to grow keeps
// its tail and has one less to grow; any other eel lets go of its tail. It
// reports whether the eel has just finished growing, which is when the
// game makes new food.
func (m *tickModelState) advance(w tickModelWay) (finishedGrowing bool) {
	if m.growth > 0 {
		m.growth--
		finishedGrowing = m.growth == 0
	} else {
		m.trail = m.trail[1:]
	}
	m.trail = append(m.trail, w.pixel)
	m.high = w.high
	return finishedGrowing
}

// tickModelFrame is the frame the model says a tick sends. A few pixels
// can rightly have either of two colors, one level apart in one channel:
// see tickModelRainbow and tickModelSignalLevels. also is the second, and
// is the same as want everywhere else.
type tickModelFrame struct {
	want, also [tickModelPixels][3]byte
	// eel marks the pixels the eel is on.
	eel [tickModelPixels]bool
}

// mark fills in f.eel and reports whether the eel is on some pixel twice,
// which ends the game.
func (m *tickModelState) mark(f *tickModelFrame) (collided bool) {
	f.eel = [tickModelPixels]bool{}
	for _, pixel := range m.trail {
		if f.eel[pixel] {
			collided = true
		}
		f.eel[pixel] = true
	}
	return collided
}

// bite lets the eel bite the food if it is on any pixel of it; f.eel has
// to be marked. A bite takes one pixel off the food, from its low end if
// the eel travels towards the high end and from its high end otherwise,
// which is the end an eel coming along the segment meets first. The first
// bite of a food is a meal: the eel has seven more to grow.
func (m *tickModelState) bite(f *tickModelFrame) {
	for position := m.foodLow; position <= m.foodHigh; position++ {
		if !f.eel[m.foodSegment*tickModelSegmentLength+position] {
			continue
		}
		if m.fresh {
			m.growth += tickModelFoodLength
			m.fresh = false
		}
		if m.high {
			m.foodLow++
		} else {
			m.foodHigh--
		}
		return
	}
}

// tickModelRainbow is the color of point i of an eel of n points, counting
// from the head: the hue goes once round the color wheel along the eel, at
// full strength whatever the brightness of the dome. Round the wheel are
// six sectors, and in each one channel is full, one is off and the third
// rises or falls in a straight line. All of it is done in whole numbers
// here, rounding down.
//
// The game does it in floating point, where a value that should be a whole
// number can come out a hair under it and round down to one less. So where
// the exact value is a whole number, one less is right as well, and also
// says so; everywhere else there is one right answer. That is 9,780 of the
// 581,581 colors eels of up to 1078 points can have, and the game is one
// level low in 4,344 of them: TestTickModelColorsOfEveryEel counts both.
func tickModelRainbow(i, n int) (want, also [3]byte) {
	sector, rest := 6*i/n, 6*i%n
	rising := rest
	if sector%2 == 1 {
		rising = n - rest
	}
	level := byte(255 * rising / n)
	low := level
	if 255*rising%n == 0 && level > 0 && level < 255 {
		low = level - 1
	}
	// Which channel is full, which one moves and which is off.
	full, moving := [6]int{0, 1, 1, 2, 2, 0}[sector], [6]int{1, 0, 2, 1, 0, 2}[sector]
	want[full], also[full] = 255, 255
	want[moving], also[moving] = level, low
	return want, also
}

// tickModelSignalLevels is how bright pixel i of a turn signal is, as a
// byte: the brightness of the dome less a tenth for every pixel from the
// junction, of 255, rounded down.
//
// Taking i tenths from the brightness is a multiplication and a
// subtraction, and Go may round them in one step where the processor can
// (arm64 does, amd64 as built here does not), which can change the last
// bit. The model works it out both ways and takes either. For the nine
// values the brightness has in a game the two agree in every case.
func tickModelSignalLevels(brightness float64, i int) (want, also byte) {
	inTwoSteps := brightness - float64(float64(i)*0.1)
	inOneStep := math.FMA(-float64(i), 0.1, brightness)
	return byte(inTwoSteps * 255), byte(inOneStep * 255)
}

// signal paints a turn signal: five pixels of the segment an eel would
// come onto at the given end, from that end inwards, in red or in yellow
// (red and green together).
func (f *tickModelFrame) signal(onto int, brightness float64, yellow bool) {
	for i := range tickModelSignalLength {
		pixel := onto/2*tickModelSegmentLength + i
		if onto%2 == 1 {
			pixel = onto/2*tickModelSegmentLength + tickModelSegmentLength - 1 - i
		}
		want, also := tickModelSignalLevels(brightness, i)
		f.want[pixel], f.also[pixel] = [3]byte{want, 0, 0}, [3]byte{also, 0, 0}
		if yellow {
			f.want[pixel][1], f.also[pixel][1] = want, also
		}
	}
}

// paint builds the frame of a tick the eel has survived, in layers. f.eel
// has to be marked, and the food has to be as it was before this tick's
// bite.
//
// Underneath, every pixel is the same grey. Then come the turn signals,
// which show where the stick leads at the junction ahead: yellow on the
// segment it leads onto, and if it leads nowhere, red on the two segments
// that up and down lead onto. On the tick that takes the eel through a
// junction the yellow is on the segment it has just come onto. The eel is
// painted over the signals and the food over the eel, in a grey as bright
// as the dome, so the pixel being bitten off still shows as food.
//
// end is the end the eel was travelling towards before the tick, and way
// is where its head went.
func (m *tickModelState) paint(d *tickModelDome, f *tickModelFrame, in tickModelInput, end int, way tickModelWay) {
	for pixel := range f.want {
		f.want[pixel] = [3]byte{0x69, 0x69, 0x69}
	}
	f.also = f.want

	if way.onto >= 0 {
		f.signal(way.onto, in.brightness, true)
	} else if onto, ok := d.steer(end, in.stick); ok {
		f.signal(onto, in.brightness, true)
	} else {
		f.signal(d.leads[end][tickModelUp], in.brightness, false)
		f.signal(d.leads[end][tickModelDown], in.brightness, false)
	}

	n := len(m.trail)
	for i := range n {
		pixel := m.trail[n-1-i]
		f.want[pixel], f.also[pixel] = tickModelRainbow(i, n)
	}

	grey := byte(in.brightness * 255)
	for position := m.foodLow; position <= m.foodHigh; position++ {
		pixel := m.foodSegment*tickModelSegmentLength + position
		f.want[pixel] = [3]byte{grey, grey, grey}
		f.also[pixel] = f.want[pixel]
	}
}

// tickModelPackets lays a frame out as the seven ArtDmx packets that carry
// it: an 18-byte header and 512 bytes of data each, three bytes for a
// pixel, 170 pixels in each of the first six and the last 58 in the
// seventh. Data bytes past a packet's last pixel are zero.
func tickModelPackets(frame *[tickModelPixels][3]byte) (packets [tickModelUniverses][tickModelPacketLen]byte) {
	for universe := range packets {
		p := &packets[universe]
		copy(p[:], "Art-Net\x00")
		p[8], p[9] = 0x00, 0x50 // ArtDmx, low byte first
		p[10], p[11] = 0, 14    // protocol version, high byte first
		p[14] = byte(universe)  // p[12], p[13] and p[15] stay 0: no sequence, port 0, net 0
		p[16], p[17] = 0x02, 0  // 512 bytes of data, high byte first
	}
	for pixel, color := range frame {
		tickModelLayPixel(&packets, pixel, color)
	}
	return packets
}

// tickModelLayPixel writes one pixel of a frame where it belongs in the
// packets.
func tickModelLayPixel(packets *[tickModelUniverses][tickModelPacketLen]byte, pixel int, color [3]byte) {
	at := tickModelHeaderLen + 3*(pixel%tickModelFullUniverse)
	copy(packets[pixel/tickModelFullUniverse][at:at+3], color[:])
}

// The levels of nearness to the eel's head that the food rules know,
// furthest first. New food goes on the furthest level that has room.
const (
	tickModelFar = iota
	tickModelOneJunctionAhead
	tickModelOnHeadSegment
	tickModelNoRoom // what a placement is called when the eel leaves no room anywhere
	tickModelFoodOutcomes
)

var tickModelFoodOutcomeNames = [tickModelFoodOutcomes]string{
	"far from the head", "one junction ahead of the head", "on the head's segment", "nowhere free, so anywhere",
}

// judgeFood holds a new food on positions start-6 to start of segment to
// the food rules, by looking at all 784 placements there are. eel marks
// the pixels of the eel, whose head is on headSegment travelling towards
// end.
//
// A placement is free if none of its seven pixels is on the eel. Each is
// on one of three levels: on the head's segment, on a segment that joins
// the end the head is travelling towards, or further away than that. The
// food has to be free and on the furthest level that has a free placement.
// Only when no placement is free at all may it be anywhere, and then it
// overlaps the eel. It returns which of those it was, or what is wrong.
func (d *tickModelDome) judgeFood(eel *[tickModelPixels]bool, headSegment, end, segment, start int) (outcome int, wrong string) {
	var level [tickModelSegments]int
	for _, onto := range d.leads[end] {
		level[onto/2] = tickModelOneJunctionAhead
	}
	level[headSegment] = tickModelOnHeadSegment

	isFree := func(s, first int) bool {
		for position := first - tickModelFoodLength + 1; position <= first; position++ {
			if eel[s*tickModelSegmentLength+position] {
				return false
			}
		}
		return true
	}
	var free [tickModelNoRoom]int
	placements := 0
	for s := range tickModelSegments {
		for first := tickModelFoodLength - 1; first < tickModelSegmentLength; first++ {
			placements++
			if isFree(s, first) {
				free[level[s]]++
			}
		}
	}
	if placements != tickModelPlacements {
		return 0, fmt.Sprintf("the model looked at %d placements, want 784", placements)
	}

	if start < tickModelFoodLength-1 || start >= tickModelSegmentLength || segment < 0 || segment >= tickModelSegments {
		return 0, fmt.Sprintf("the food's first point is at position %d of segment %d, which is none of the 784 placements", start, segment)
	}
	at := fmt.Sprintf("the new food is on %s positions %d to %d", d.labels[segment], start-tickModelFoodLength+1, start)
	if free[tickModelFar]+free[tickModelOneJunctionAhead]+free[tickModelOnHeadSegment] == 0 {
		return tickModelNoRoom, ""
	}
	if !isFree(segment, start) {
		return 0, fmt.Sprintf("%s, on the eel, while %d placements far from the head, %d one junction ahead and %d on the head's segment are free",
			at, free[tickModelFar], free[tickModelOneJunctionAhead], free[tickModelOnHeadSegment])
	}
	for best, n := range free {
		if n == 0 {
			continue
		}
		if level[segment] != best {
			return 0, fmt.Sprintf("%s, %s (%s), while %d placements %s are free",
				at, tickModelFoodOutcomeNames[level[segment]], d.labels[headSegment], n, tickModelFoodOutcomeNames[best])
		}
		return best, ""
	}
	panic("unreachable: some placement is free")
}

// tickModelStats counts what a run went through.
type tickModelStats struct {
	ticks, games        int
	junctions           int // ticks that took the eel through a junction
	demoTurns           int // of those, the ones where the game picked the turn
	noWayOn, collisions int // ticks that ended a game, each of the two ways
	meals               int
	newFood             [tickModelFoodOutcomes]int // food made by a tick, by where the rules had it go
	eitherColor         int                        // pixels that could rightly be one of two colors
	longest             int
}

func (s *tickModelStats) add(o tickModelStats) {
	s.ticks += o.ticks
	s.games += o.games
	s.junctions += o.junctions
	s.demoTurns += o.demoTurns
	s.noWayOn += o.noWayOn
	s.collisions += o.collisions
	s.meals += o.meals
	for i, n := range o.newFood {
		s.newFood[i] += n
	}
	s.eitherColor += o.eitherColor
	s.longest = max(s.longest, o.longest)
}

func (s tickModelStats) String() string {
	return fmt.Sprintf("%d ticks in %d games: %d junctions (%d turns picked by the game), %d meals, "+
		"%d new foods (%d far from the head, %d one junction ahead, %d on its segment, %d with no room), "+
		"%d ticks ended a game for want of a way on and %d by the eel running into itself, longest eel %d points",
		s.ticks, s.games, s.junctions, s.demoTurns, s.meals,
		s.newFood[0]+s.newFood[1]+s.newFood[2]+s.newFood[3], s.newFood[0], s.newFood[1], s.newFood[2], s.newFood[3],
		s.noWayOn, s.collisions, s.longest)
}

// tickModelRun plays a real game and the model side by side.
type tickModelRun struct {
	t    *testing.T
	g    *Game
	dome *tickModelDome
	m    tickModelState

	// name says which run this is in a failure message, with whatever is
	// needed to run it again.
	name string

	// wire, if set, receives the game's frames, and the datagrams are
	// compared too.
	wire *artnetSink

	// anyGrowth is set by a run that gives an eel more to grow than a
	// game can, which takes the check of that range away.
	anyGrowth bool

	// scattered is set by a run whose eels are made up pixel by pixel and
	// are not in one piece, which takes the check of that away.
	scattered bool

	// length is the eel's points and what it had left to grow when the
	// game started, and meals the foods it has eaten since, as seen from
	// the game's own food going stale.
	length, meals int

	// sent is the packets of the last frame the game sent.
	sent [tickModelUniverses][tickModelPacketLen]byte

	// graph is the Points that belong to the game's segment table.
	graph map[*Point]bool

	// What the tick being checked started from, for failure messages, and
	// whether it is a tick at all and not the start of a game.
	before tickModelState
	input  tickModelInput
	inTick bool

	frame tickModelFrame
	stats tickModelStats
}

// tickModelHops returns the eight hops of a segment of the game: the four
// turns for leaving by the low end, then the four for the high end, each
// in the model's order of turns.
func tickModelHops(s *Segment) [2][tickModelTurns]*Hop {
	return [2][tickModelTurns]*Hop{
		{s.LesserUp, s.LesserDown, s.LesserLeft, s.LesserRight},
		{s.GreaterUp, s.GreaterDown, s.GreaterLeft, s.GreaterRight},
	}
}

// tickModelNewRun gets a game ready to be played beside the model. The
// game must have just been made: its packets are still the headers and no
// pixels.
func tickModelNewRun(t *testing.T, g *Game, name string) *tickModelRun {
	t.Helper()
	r := &tickModelRun{t: t, g: g, dome: tickModelReadDome(t), name: name, graph: map[*Point]bool{}}
	r.checkGraph()
	for _, s := range g.Segments {
		for _, hops := range tickModelHops(s) {
			for _, hop := range hops {
				r.graph[hop.Point] = true
			}
		}
	}
	var dark [tickModelPixels][3]byte
	r.sent = tickModelPackets(&dark)
	r.checkPacketsUnchanged("before the first tick")
	return r
}

// failf stops the test, saying which run and tick it was and what the
// tick started from.
//
// Neither it nor the checks that run on every tick call t.Helper: that
// walks the stack on every call, which took two thirds of the time of a
// run. A failure is therefore reported at this line, and the message has
// to say what was being checked.
func (r *tickModelRun) failf(format string, args ...any) {
	again := ""
	if r.inTick {
		again = fmt.Sprintf("\n\tto play this tick again: dev/run go test ./game/ -run TestTickModelPlaysATickAgain -v -tick-model-tick='%s'", r.again())
	}
	r.t.Fatalf("%s, tick %d: %s\n\tplayed by: %v\n\tfrom: %s%s",
		r.name, r.stats.ticks, fmt.Sprintf(format, args...), r.input, r.before.describe(r.dome), again)
}

// checkGraph compares the game's segment table with the listing the model
// reads: 49 segments, each with its label and its 22 pixels of the frame,
// and every hop leading where the listing says. A Point of the table that
// got into the eel's body is written to when it comes round to being the
// head again, and a hop then leads somewhere else, which shows here.
func (r *tickModelRun) checkGraph() {
	r.t.Helper()
	g, d := r.g, r.dome
	if len(g.Segments) != tickModelSegments {
		r.t.Fatalf("%s: the game has %d segments, want 49", r.name, len(g.Segments))
	}
	for i, s := range g.Segments {
		if s == nil || s.Label != d.labels[i] || s.Offset != i*tickModelSegmentLength {
			r.t.Fatalf("%s: segment %d of the game is not %s starting at pixel %d", r.name, i, d.labels[i], i*tickModelSegmentLength)
		}
		for high, hops := range tickModelHops(s) {
			for turn, hop := range hops {
				want := tickModelArriving(d.leads[2*i+high][turn])
				if hop == nil || hop.Point == nil || hop.Point.Segment != g.Segments[want.pixel/tickModelSegmentLength] ||
					hop.Point.Position != want.pixel%tickModelSegmentLength || (hop.Direction == Greater) != want.high {
					r.t.Fatalf("%s: the hop of %s for the stick %s, leaving by the %s end, does not lead to %s as the listing of the dome says",
						r.name, s.Label, [tickModelTurns]string{"up", "down", "left", "right"}[turn], [2]string{"low", "high"}[high], d.where(want.pixel))
				}
			}
		}
	}
}

// pixel returns the pixel a Point of the game is on. The Point has to be
// on one of the game's own segments, at one of its 22 positions: position
// 22 of one segment is the same number as position 0 of the next, and
// must not pass for it.
//
// The Point is point i of what, the eel or a food, for a failure message.
func (r *tickModelRun) pixel(p *Point, what string, i int) int {
	if p == nil || p.Segment == nil {
		r.failf("point %d of %s is missing or is on no segment", i, what)
	}
	segment := p.Segment.Offset / tickModelSegmentLength
	if p.Segment.Offset%tickModelSegmentLength != 0 || segment < 0 || segment >= tickModelSegments || r.g.Segments[segment] != p.Segment {
		r.failf("point %d of %s is on a segment (%q, pixels from %d) that is not one of the game's 49", i, what, p.Segment.Label, p.Segment.Offset)
	}
	if p.Position < 0 || p.Position >= tickModelSegmentLength {
		r.failf("point %d of %s is at position %d of %s, which has positions 0 to 21", i, what, p.Position, p.Segment.Label)
	}
	return segment*tickModelSegmentLength + p.Position
}

// points makes Points of the game for the given pixels.
func (r *tickModelRun) points(pixels []int, capacity int) []*Point {
	points := make([]*Point, len(pixels), max(capacity, len(pixels)))
	for i, pixel := range pixels {
		points[i] = &Point{Segment: r.g.Segments[pixel/tickModelSegmentLength], Position: pixel % tickModelSegmentLength}
	}
	return points
}

// set puts the game in the state m, with an eel and a food made for it,
// and leaves the count of games and meals alone: it is how a run goes back
// to a state it was in before. The eel's body gets the room a new eel's
// has.
func (r *tickModelRun) set(m tickModelState) {
	r.m = m.clone()
	g := r.g
	headFirst := make([]int, len(m.trail))
	for i, pixel := range m.trail {
		headFirst[len(m.trail)-1-i] = pixel
	}
	travel := Lesser
	if m.high {
		travel = Greater
	}
	g.Eel = &Eel{
		Body:      r.points(headFirst, 256),
		Game:      g,
		Growth:    m.growth,
		Segment:   g.Segments[m.head()/tickModelSegmentLength],
		TravelDir: travel,
	}
	var food []int
	for position := m.foodHigh; position >= m.foodLow; position-- {
		food = append(food, m.foodSegment*tickModelSegmentLength+position)
	}
	g.Food = &Food{Body: r.points(food, 0), Game: g, Fresh: m.fresh}
}

// start starts a game from the state m.
func (r *tickModelRun) start(m tickModelState) {
	r.set(m)
	r.stats.games++
	r.length, r.meals = len(m.trail)+m.growth, 0
	r.before, r.input, r.inTick = m.clone(), tickModelInput{}, false
}

// startAsRunDoes starts a game the way Game.Run does, with the game's own
// NewEel and NewFood, and has the model take over what they made after
// checking it: a new eel is seven points at the low end of a segment with
// its head at position 6, travelling towards the high end with nothing to
// grow, and the first food obeys the food rules like any other.
func (r *tickModelRun) startAsRunDoes() {
	r.t.Helper()
	g := r.g
	g.Eel = NewEel(g)
	g.Food = NewFood(g)
	r.stats.games++
	r.before, r.input, r.inTick = tickModelState{}, tickModelInput{}, false

	eel := g.Eel
	if len(eel.Body) != tickModelFoodLength || eel.Growth != 0 || eel.TravelDir != Greater || eel.Game != g {
		r.failf("a new eel has %d points, %d to grow and travels %d, want 7 points, nothing to grow and towards the high end",
			len(eel.Body), eel.Growth, eel.TravelDir)
	}
	r.m = tickModelState{high: true, trail: r.m.trail[:0]}
	for i := len(eel.Body) - 1; i >= 0; i-- {
		r.m.trail = append(r.m.trail, r.pixel(eel.Body[i], "a new eel", i))
	}
	segment := r.m.head() / tickModelSegmentLength
	for i, pixel := range r.m.trail {
		if pixel != segment*tickModelSegmentLength+i {
			r.failf("a new eel has its point %d on %s, want positions 6 down to 0 of one segment", len(r.m.trail)-1-i, r.dome.where(pixel))
		}
	}
	if eel.Segment != g.Segments[segment] {
		r.failf("a new eel says it is on %s, and its head is on %s", eel.Segment.Label, r.dome.labels[segment])
	}
	r.length, r.meals = tickModelFoodLength, 0
	r.takeNewFood(false)
}

// takeNewFood has the model take over the food the game has just made,
// after holding it to the food rules. The eel has to be where it is when
// the food is made: Move makes it after the head has moved.
//
// A food made where there was no room overlaps the eel and is bitten on
// the same tick, so by the time it can be looked at it has six points and
// is not fresh. bitten says the model's eel is alive to bite; the food is
// then put back together as it was made, and the model bites it itself
// afterwards.
func (r *tickModelRun) takeNewFood(bitten bool) (outcome int) {
	g, m := r.g, &r.m
	food := g.Food
	if food == nil || len(food.Body) == 0 {
		r.failf("the new food has no points")
	}
	high := r.pixel(food.Body[0], "the new food", 0)
	segment, start := high/tickModelSegmentLength, high%tickModelSegmentLength
	low := start - len(food.Body) + 1
	for i, p := range food.Body {
		if r.pixel(p, "the new food", i) != high-i {
			r.failf("the new food is not points in a row on one segment, from its highest position down")
		}
	}
	switch {
	case len(food.Body) == tickModelFoodLength && food.Fresh:
	case len(food.Body) == tickModelFoodLength-1 && !food.Fresh && bitten:
		// The bite took the low end if the eel travels towards the high
		// end, and the high end otherwise.
		if m.high {
			low--
		} else {
			start++
		}
	default:
		r.failf("the new food has %d points and fresh is %v, want 7 fresh points", len(food.Body), food.Fresh)
	}

	end, _ := m.leaving()
	m.mark(&r.frame)
	outcome, wrong := r.dome.judgeFood(&r.frame.eel, m.head()/tickModelSegmentLength, end, segment, start)
	if wrong != "" {
		r.failf("%s", wrong)
	}
	if !food.Fresh && outcome != tickModelNoRoom {
		r.failf("the new food was bitten on the tick it appeared, though it was put where the eel is not")
	}
	m.foodSegment, m.foodLow, m.foodHigh, m.fresh = segment, low, start, true
	return outcome
}

// tick plays one tick on the game and on the model and compares them. It
// reports whether the game is over.
func (r *tickModelRun) tick(in tickModelInput) (over bool) {
	g, m, d := r.g, &r.m, r.dome
	r.stats.ticks++
	r.before.trail = append(r.before.trail[:0], m.trail...)
	r.before.growth, r.before.high = m.growth, m.high
	r.before.foodSegment, r.before.foodLow, r.before.foodHigh, r.before.fresh = m.foodSegment, m.foodLow, m.foodHigh, m.fresh
	r.input, r.inTick = in, true

	// The real tick. The input goes in as the joystick's goroutine leaves
	// it, as one word.
	snapshot := stickHeld(in.stick).withDemo(in.demo)
	if in.lost {
		snapshot |= inputJoystickLost
	}
	g.input.Swap(snapshot)
	g.Brightness = in.brightness
	foodBefore, wasFresh := g.Food, g.Food.Fresh
	err := g.Eel.Move()

	// Meals are counted from what the game does to its own food, apart
	// from the model: a food that was fresh and is not, or a new one that
	// is stale already.
	if (g.Food == foodBefore && wasFresh && !g.Food.Fresh) || (g.Food != foodBefore && !g.Food.Fresh) {
		r.meals++
		r.stats.meals++
	}

	// The model's tick.
	end, atJunction := m.leaving()
	ways, n := m.ways(d, in)
	if n == 0 {
		// Nowhere to go ends the game, and such a tick changes nothing
		// and sends nothing.
		if err == nil {
			r.failf("Move went on, to %s, where the stick leads nowhere and the game is over",
				d.where(r.pixel(g.Eel.Body[0], "the eel", 0)))
		}
		if g.Food != foodBefore {
			r.failf("the tick that ended the game for want of a way on made new food")
		}
		r.compareState()
		r.checkPacketsUnchanged("on a tick that ended the game")
		r.checkInvariants(false)
		r.stats.noWayOn++
		return true
	}

	// Where the game picked the turn, the model goes the way it went.
	gotHead, gotHigh := r.pixel(g.Eel.Body[0], "the eel", 0), g.Eel.TravelDir == Greater
	way := -1
	for i, w := range ways[:n] {
		if w.pixel == gotHead && w.high == gotHigh {
			way = i
		}
	}
	if way < 0 {
		allowed := make([]string, n)
		for i, w := range ways[:n] {
			allowed[i] = d.where(w.pixel)
		}
		r.failf("Move (which returned %v) left the head on %s travelling towards the high end=%v; the rules take it to %s",
			err, d.where(gotHead), gotHigh, strings.Join(allowed, " or "))
	}

	finishedGrowing := m.advance(ways[way])
	collided := m.mark(&r.frame)
	switch {
	case finishedGrowing && g.Food == foodBefore:
		r.failf("the eel finished growing and Move made no new food")
	case finishedGrowing:
		r.stats.newFood[r.takeNewFood(!collided)]++
	case g.Food != foodBefore:
		r.failf("Move made new food though the eel had not just finished growing")
	}

	if collided {
		// The eel ran into itself. Its head is where it went, which the
		// game-over animation starts from, but the eel does not eat and
		// no frame is sent. What the tick left in the game's frame buffer
		// is never shown, and is not compared.
		if err == nil {
			r.failf("Move let the eel onto a pixel it is already on and went on with the game")
		}
		r.compareState()
		r.checkPacketsUnchanged("on a tick that ended the game")
		r.checkInvariants(true)
		r.stats.collisions++
		return true
	}
	if err != nil {
		r.failf("Move ended the game (%v) with the eel on %d pixels of its own and a way on", err, len(m.trail))
	}

	m.paint(d, &r.frame, in, end, ways[way])
	m.bite(&r.frame)
	r.compareState()
	r.compareFrame()
	r.checkInvariants(false)

	if atJunction {
		r.stats.junctions++
		if in.stick == controller.Dir_Neutral {
			r.stats.demoTurns++
		}
	}
	r.stats.longest = max(r.stats.longest, len(m.trail))
	return false
}

// compareState compares the eel and the food of the game with the model's.
func (r *tickModelRun) compareState() {
	g, m, d := r.g, &r.m, r.dome
	eel, n := g.Eel, len(m.trail)
	if len(eel.Body) != n {
		r.failf("the eel has %d points, the model %d", len(eel.Body), n)
	}
	for i, p := range eel.Body {
		if got, want := r.pixel(p, "the eel", i), m.trail[n-1-i]; got != want {
			r.failf("point %d of the eel's %d (0 is the head) is on %s, the model has it on %s", i, n, d.where(got), d.where(want))
		}
	}
	if eel.Segment != g.Segments[m.head()/tickModelSegmentLength] {
		r.failf("the eel says it is on segment %s, and its head is on %s", eel.Segment.Label, d.where(m.head()))
	}
	if (eel.TravelDir == Greater) != m.high || (eel.TravelDir != Greater && eel.TravelDir != Lesser) {
		r.failf("the eel travels in direction %d (0 is towards the high end), the model towards the high end=%v", eel.TravelDir, m.high)
	}
	if eel.Growth != m.growth {
		r.failf("the eel has %d left to grow, the model %d", eel.Growth, m.growth)
	}

	food, left := g.Food, max(0, m.foodHigh-m.foodLow+1)
	if len(food.Body) != left || food.Fresh != m.fresh {
		r.failf("the food has %d points left (fresh=%v), the model %d (fresh=%v)", len(food.Body), food.Fresh, left, m.fresh)
	}
	for i, p := range food.Body {
		want := m.foodSegment*tickModelSegmentLength + m.foodHigh - i
		if got := r.pixel(p, "the food", i); got != want {
			r.failf("point %d of the food's %d is on %s, the model has it on %s", i, left, d.where(got), d.where(want))
		}
	}
}

// compareFrame compares what a tick the eel survived drew and sent: every
// pixel of the frame with the model's, the map of the eel's pixels, and
// the seven packets, byte for byte, with the frame laid out the model's
// way. With a wire it also reads the seven datagrams back and compares
// those.
func (r *tickModelRun) compareFrame() {
	g, f := r.g, &r.frame
	if len(g.frame) != tickModelPixels || len(g.frameSet) != tickModelPixels {
		r.failf("the frame has %d pixels and the map of the eel %d, want 1078", len(g.frame), len(g.frameSet))
	}
	// The packets are the last frame's with every pixel written again.
	// Their headers and the zeros after each one's last pixel stay as
	// tickModelPackets made them.
	for pixel := range f.want {
		got := [3]byte(g.frame[pixel])
		if got != f.want[pixel] && got != f.also[pixel] {
			wanted := fmt.Sprint(f.want[pixel])
			if f.also[pixel] != f.want[pixel] {
				wanted += fmt.Sprintf(" or %v", f.also[pixel])
			}
			r.failf("pixel %d of the frame (%s) is %v, the model has %s", pixel, r.dome.where(pixel), got, wanted)
		}
		if f.also[pixel] != f.want[pixel] {
			r.stats.eitherColor++
		}
		if g.frameSet[pixel] != f.eel[pixel] {
			r.failf("the map of the eel's pixels says %v for pixel %d (%s), the model %v", g.frameSet[pixel], pixel, r.dome.where(pixel), f.eel[pixel])
		}
		tickModelLayPixel(&r.sent, pixel, got)
	}

	if len(g.artnetBufs) != tickModelUniverses {
		r.failf("the game keeps %d packets, want 7", len(g.artnetBufs))
	}
	for universe := range r.sent {
		r.compareBytes(g.artnetBufs[universe][:], universe, "the packet built")
	}
	if r.wire != nil {
		for universe := range r.sent {
			datagram := r.wire.datagram(time.Second)
			if datagram == nil {
				r.failf("datagram %d of the frame's 7 was not sent", universe)
			}
			r.compareBytes(datagram, universe, "the datagram sent")
		}
	}
}

// compareBytes compares one packet with the model's for that universe.
func (r *tickModelRun) compareBytes(got []byte, universe int, what string) {
	want := r.sent[universe][:]
	if bytes.Equal(got, want) {
		return
	}
	at := 0
	for at < len(got) && at < len(want) && got[at] == want[at] {
		at++
	}
	switch {
	case at == len(got) || at == len(want):
		r.failf("%s for universe %d is %d bytes, want 530", what, universe, len(got))
	case at < tickModelHeaderLen:
		r.failf("%s for universe %d has %#02x at byte %d of its header, want %#02x", what, universe, got[at], at, want[at])
	}
	inData := at - tickModelHeaderLen
	r.failf("%s for universe %d has %#02x at byte %d, want %#02x: the frame's pixel %d belongs at bytes %d to %d of this packet",
		what, universe, got[at], at, want[at], universe*tickModelFullUniverse+inData/3, at-inData%3, at-inData%3+2)
}

// checkPacketsUnchanged checks that the game's packets are still those of
// the last frame it sent.
func (r *tickModelRun) checkPacketsUnchanged(when string) {
	for universe := range r.sent {
		if !bytes.Equal(r.g.artnetBufs[universe][:], r.sent[universe][:]) {
			r.failf("the packet for universe %d changed %s, when no frame is sent", universe, when)
		}
	}
}

// checkInvariants checks what must hold of the game after every tick,
// whatever the model says. It reads the game only, and the dome's links.
//
//   - The eel has between 0 and 7 left to grow.
//   - The eel is all in one piece: each point is the pixel next to the
//     one behind it on the same segment, or across a junction from it.
//     (Not looked at in a run that makes up eels in pieces.)
//   - No two points of the eel are on one pixel, unless this tick ended
//     the game by the eel running into itself, and then only its head is
//     on a pixel twice.
//   - The eel's length and what it has left to grow add up to what it
//     started with and seven for every meal.
//   - A food that has been bitten leaves the eel growing, so the eel will
//     finish growing and new food will come.
//   - The eel's points are its own: none is a Point of the segment table,
//     and none is in the body twice.
func (r *tickModelRun) checkInvariants(collided bool) {
	g, d := r.g, r.dome
	eel := g.Eel
	if eel.Growth < 0 || (eel.Growth > tickModelFoodLength && !r.anyGrowth) {
		r.failf("the eel has %d left to grow, outside 0 to 7", eel.Growth)
	}
	if got, want := len(eel.Body)+eel.Growth, r.length+tickModelFoodLength*r.meals; got != want {
		r.failf("the eel is %d points with %d to grow, %d in all; it started with %d and has had %d meals, which makes %d",
			len(eel.Body), eel.Growth, got, r.length, r.meals, want)
	}
	if !g.Food.Fresh && eel.Growth == 0 {
		r.failf("the food has been bitten and the eel has nothing left to grow, so no new food would ever come")
	}

	var on [tickModelPixels]int
	twice := 0
	for i, p := range eel.Body {
		pixel := r.pixel(p, "the eel", i)
		on[pixel]++
		if on[pixel] > 1 {
			twice++
		}
		if i == 0 {
			continue
		}
		// The point ahead of this one is the next pixel of the segment,
		// or this one is an end pixel with a link onto the pixel ahead.
		ahead := r.pixel(eel.Body[i-1], "the eel", i-1)
		linked := false
		if ahead/tickModelSegmentLength == pixel/tickModelSegmentLength {
			linked = ahead-pixel == 1 || pixel-ahead == 1
		} else if position := pixel % tickModelSegmentLength; position == 0 || position == tickModelSegmentLength-1 {
			end := 2 * (pixel / tickModelSegmentLength)
			if position != 0 {
				end++
			}
			for _, onto := range d.leads[end] {
				linked = linked || tickModelArriving(onto).pixel == ahead
			}
		}
		if !linked && !r.scattered {
			r.failf("point %d of the eel is on %s and point %d on %s, which are not next to each other on the dome",
				i-1, d.where(ahead), i, d.where(pixel))
		}
	}
	switch {
	case !collided && twice > 0:
		r.failf("%d points of the eel are on a pixel another point is on, and the game goes on", twice)
	case collided && (twice != 1 || on[r.pixel(eel.Body[0], "the eel", 0)] != 2):
		r.failf("the game ended by the eel running into itself, but it is not the head alone that is on a pixel twice")
	}

	// Points come into the body at the head, so the head is looked at on
	// every tick, and the whole body now and then and when a game ends.
	if r.graph[eel.Body[0]] {
		r.failf("the eel's head is a Point of the segment table, which the next tick that reuses it will write to")
	}
	if collided || r.stats.ticks%64 == 0 {
		seen := make(map[*Point]bool, len(eel.Body))
		for i, p := range eel.Body {
			if r.graph[p] {
				r.failf("point %d of the eel is a Point of the segment table", i)
			}
			if seen[p] {
				r.failf("point %d of the eel is the same Point as an earlier one", i)
			}
			seen[p] = true
		}
	}
}
