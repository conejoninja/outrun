package main

import (
	_ "embed"
	"image/color"
	"machine"
	"math/rand"
	"strconv"
	"time"

	pio "github.com/tinygo-org/pio/rp2-pio"
	"github.com/tinygo-org/pio/rp2-pio/piolib"

	"github.com/conejoninja/gamelink/gamelink"
	"tinygo.org/x/drivers"
	"tinygo.org/x/drivers/encoders"
	"tinygo.org/x/drivers/pixel"
	"tinygo.org/x/drivers/ssd1306"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/proggy"
)

const (
	MENU = iota
	START
	LOOP
	LOSE
	LOAD
	INFO
)

const (
	KEY_PRESSED = 12
)

var (
	textWhite = color.RGBA{255, 255, 255, 255}
	textBlack = color.RGBA{0, 0, 0, 255}

	rotaryOldValue, rotaryNewValue int

	state = MENU

	colPins = []machine.Pin{
		machine.GPIO5,
		machine.GPIO6,
		machine.GPIO7,
		machine.GPIO8,
	}

	rowPins = []machine.Pin{
		machine.GPIO9,
		machine.GPIO10,
		machine.GPIO11,
	}

	matrixBtn [12]bool
	colors    []uint32

	carLane    = 1
	framesLoop = 180
	frame      = 0
	obstacles  [4][3]bool
	pressed    = false
	score      = 0
)

const (
	white = 0x3F3F3FFF
	red   = 0x00FF00FF
	green = 0xFF0000FF
	blue  = 0x0000FFFF
	black = 0x000000FF
)

type WS2812B struct {
	Pin machine.Pin
	ws  *piolib.WS2812B
}

//go:embed title.bin
var title []byte

func NewWS2812B(pin machine.Pin) *WS2812B {
	s, _ := pio.PIO0.ClaimStateMachine()
	ws, _ := piolib.NewWS2812B(s, pin)
	ws.EnableDMA(true)
	return &WS2812B{
		ws: ws,
	}
}

func (ws *WS2812B) WriteRaw(rawGRB []uint32) error {
	return ws.ws.WriteRaw(rawGRB)
}

func main() {

	i2c := machine.I2C0
	i2c.Configure(machine.I2CConfig{
		Frequency: 2.8 * machine.MHz,
		SDA:       machine.GPIO12,
		SCL:       machine.GPIO13,
	})

	display := ssd1306.NewI2C(i2c)
	display.Configure(ssd1306.Config{
		Address:  0x3C,
		Width:    128,
		Height:   64,
		Rotation: drivers.Rotation180,
	})
	display.ClearDisplay()

	gl := gamelink.New(i2c)
	gl.Configure()

	enc := encoders.NewQuadratureViaInterrupt(
		machine.GPIO4,
		machine.GPIO3,
	)

	enc.Configure(encoders.QuadratureConfig{
		Precision: 4,
	})
	rotaryBtn := machine.GPIO2
	rotaryBtn.Configure(machine.PinConfig{Mode: machine.PinInputPullup})

	for _, c := range colPins {
		c.Configure(machine.PinConfig{Mode: machine.PinOutput})
		c.Low()
	}

	for _, c := range rowPins {
		c.Configure(machine.PinConfig{Mode: machine.PinInputPulldown})
	}

	colors = []uint32{
		black, black, black,
		black, black, black,
		black, black, black,
		black, black, black,
	}
	ws := NewWS2812B(machine.GPIO1)
	menuOption := int16(0)

	for {
		display.ClearBuffer()

		getMatrixState()

		switch state {
		case LOAD:
			img := pixel.NewImageFromBytes[pixel.Monochrome](96, 64, []byte(title))
			if err := display.DrawBitmap(32, 0, img); err != nil {
				println(err)
			}
			display.Display()
			time.Sleep(2 * time.Second)
			state = MENU
			break

		case MENU:

			if rotaryNewValue = enc.Position(); rotaryNewValue != rotaryOldValue {
				if rotaryNewValue > rotaryOldValue {
					menuOption = 1
				} else {
					menuOption = 0
				}
				rotaryOldValue = rotaryNewValue
			}

			if menuOption == 0 {
				tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 10, 20, "[+] START GAME", textWhite)
				tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 10, 34, "[ ] INFO", textWhite)
			} else {
				tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 10, 20, "[ ] START GAME", textWhite)
				tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 10, 34, "[+] INFO", textWhite)
			}

			if !rotaryBtn.Get() {
				if !pressed {
					if menuOption == 0 {

						for i := 0; i < 4; i++ {
							for j := 0; j < 3; j++ {
								obstacles[i][j] = false
								colors[3*i+j] = black
								carLane = 1
							}
						}

						colors[9+carLane] = red
						score = 0

						state = LOOP
					} else {
						pressed = true
						menuOption = 0
						state = INFO
					}
				}
			} else {
				pressed = false
			}
			break
		case INFO:
			if rotaryNewValue = enc.Position(); rotaryNewValue != rotaryOldValue {
				if rotaryNewValue > rotaryOldValue {
					menuOption -= 4
					if menuOption < -14*8 {
						menuOption = -14 * 8
					}
				} else {
					menuOption += 4
					if menuOption > 0 {
						menuOption = 0
					}
				}
				rotaryOldValue = rotaryNewValue

			}

			tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 0, menuOption+20, "--- OUTRUN ---", textWhite)
			tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 0, menuOption+34, "In this game you", textWhite)
			tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 0, menuOption+48, "drive a RED car and", textWhite)
			tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 0, menuOption+62, "use the know to", textWhite)
			tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 0, menuOption+76, "change lanes. Avoid", textWhite)
			tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 0, menuOption+90, "the green trees. To", textWhite)
			tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 0, menuOption+104, "better play rotate", textWhite)
			tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 0, menuOption+118, "your keeb 90.", textWhite)
			tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 0, menuOption+132, "degrees clockwise.", textWhite)
			tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 0, menuOption+160, "-- Press KNOB --", textWhite)

			if !rotaryBtn.Get() {
				if !pressed {
					pressed = true
					menuOption = 0
					state = MENU
				}
			} else {
				pressed = false
			}

			break
		case LOOP:

			frame++
			score++
			if frame > framesLoop {
				frame = 0
				for i := 3; i > 0; i-- {
					for j := 0; j < 3; j++ {
						obstacles[i][j] = obstacles[i-1][j]
					}
				}
				obstacles[0][0] = false
				obstacles[0][1] = false
				obstacles[0][2] = false
				if obstacles[3][carLane] {
					state = LOSE
				}
				r := rand.Int31n(6)
				if r < 3 {
					obstacles[0][r] = true
				}
				for i := 0; i < 4; i++ {
					for j := 0; j < 3; j++ {
						if obstacles[i][j] {
							colors[3*i+j] = green
						} else {
							colors[3*i+j] = black
						}
					}
				}
				colors[9+carLane] = red
				framesLoop = (98 * framesLoop) / 100
			}

			if rotaryNewValue = enc.Position(); rotaryNewValue != rotaryOldValue {
				colors[9+carLane] = black
				if rotaryNewValue > rotaryOldValue {
					carLane--
				} else {
					carLane++
				}
				rotaryOldValue = rotaryNewValue
				if carLane < 0 {
					carLane = 0
				}
				if carLane >= 2 {
					carLane = 2
				}
				colors[9+carLane] = red
				if obstacles[3][carLane] {
					pressed = true
					state = LOSE
				}
			}

			tinyfont.WriteLineRotated(&display, &proggy.TinySZ8pt7b, 20, 60, "SCORE", textWhite, tinyfont.ROTATION_270)
			tinyfont.WriteLineRotated(&display, &proggy.TinySZ8pt7b, 34, 64, strconv.Itoa(score), textWhite, tinyfont.ROTATION_270)

			break
		case LOSE:
			tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 10, 20, "You LOSE", textWhite)
			tinyfont.WriteLine(&display, &proggy.TinySZ8pt7b, 10, 34, "SCORE:"+strconv.Itoa(score), textWhite)

			if !rotaryBtn.Get() {
				if !pressed {
					pressed = true
					menuOption = 0
					state = MENU
				}
			} else {
				pressed = false
			}
			break
		}

		ws.WriteRaw(colors)
		display.Display()
		time.Sleep(time.Millisecond)
	}

}

func getMatrixState() {
	colPins[0].High()
	colPins[1].Low()
	colPins[2].Low()
	colPins[3].Low()
	time.Sleep(1 * time.Millisecond)

	matrixBtn[0] = rowPins[0].Get()
	matrixBtn[1] = rowPins[1].Get()
	matrixBtn[2] = rowPins[2].Get()

	// COL2
	colPins[0].Low()
	colPins[1].High()
	colPins[2].Low()
	colPins[3].Low()
	time.Sleep(1 * time.Millisecond)

	matrixBtn[3] = rowPins[0].Get()
	matrixBtn[4] = rowPins[1].Get()
	matrixBtn[5] = rowPins[2].Get()

	// COL3
	colPins[0].Low()
	colPins[1].Low()
	colPins[2].High()
	colPins[3].Low()
	time.Sleep(1 * time.Millisecond)

	matrixBtn[6] = rowPins[0].Get()
	matrixBtn[7] = rowPins[1].Get()
	matrixBtn[8] = rowPins[2].Get()

	// COL4
	colPins[0].Low()
	colPins[1].Low()
	colPins[2].Low()
	colPins[3].High()
	time.Sleep(1 * time.Millisecond)

	matrixBtn[9] = rowPins[0].Get()
	matrixBtn[10] = rowPins[1].Get()
	matrixBtn[11] = rowPins[2].Get()
}
