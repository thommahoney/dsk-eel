package main

import (
	"fmt"
	"log"

	"github.com/sstallion/go-hid"
)

func main() {
	debug := false

	// Initialize the hid package.
	if err := hid.Init(); err != nil {
		log.Fatal(err)
	}

	// Open the device using the VID and PID.
	// d, err := hid.OpenFirst(0x4d8, 0x3f)
	d, err := hid.OpenPath("/dev/hidraw3")
	if err != nil {
		log.Fatal(err)
	}

	// Read the Manufacturer String.
	s, err := d.GetMfrStr()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Manufacturer String: %s\n", s)

	// Read the Product String.
	s, err = d.GetProductStr()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Product String: %s\n", s)

	up, down, left, right := false, false, false, false
	white, red, yellow, blue := false, false, false, false
	var prevButtons byte = 15>>4

	b := make([]byte, 8)
	for {
		_, err := d.Read(b)
		if err != nil {
			log.Fatal(err)
		}

		jsChange := false
		updown, leftright := b[0], b[1]

		switch updown {
		case 0:
			if !down {
				jsChange = true
			}
			down = true
		case 127:
			if down || up {
				jsChange = true
			}
			down, up = false, false
		case 255:
			if !up {
				jsChange = true
			}
			up = true
		}

		switch leftright {
		case 0:
			if !left {
				jsChange = true
			}
			left = true
		case 127:
			if left || right {
				jsChange = true
			}
			left, right = false, false
		case 255:
			if !right {
				jsChange = true
			}
			right = true
		}

		if jsChange {
			fmt.Printf("joystick: ")
			if debug {
				fmt.Printf("[%08b %08b] ", updown, leftright)
			}
			if !up && !down && !left && !right {
				fmt.Println("neutral")
				continue
			}
			if up {
				fmt.Printf("north")
			} else if down {
				fmt.Printf("south")
			}
			if left {
				fmt.Println("west")
			} else if right {
				fmt.Println("east")
			} else {
				fmt.Println("")
			}
		}

		buttons := b[5]>>4
		bChange := false
		if buttons != prevButtons {
			bChange = true
			prevButtons = buttons
		}

		if buttons & 0b0001 > 0 {
			white = true
		} else {
			white = false
		}

		if buttons & 0b0010 > 0 {
			red = true
		} else {
			red = false
		}

		if buttons & 0b0100 > 0 {
			blue = true
		} else {
			blue = false
		}

		if buttons & 0b1000 > 0 {
			yellow = true
		} else {
			yellow = false
		}

		if bChange {
			fmt.Printf("buttons: ")
			if debug {
				fmt.Printf("[%04b] ", buttons)
			}
			if white {
				fmt.Print("⚪️")
			}
			if red {
				fmt.Print("️🔴")
			}
			if yellow {
				fmt.Print("🟡")
			}
			if blue {
				fmt.Print("🔵")
			}
			fmt.Println("")
		}
	}
}
