package main

import (
	//	"github.com/as/clip"
	"fmt"
	"golang.org/x/exp/shiny/driver"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"
	"image"
	"image/draw"
	"io"
	"os"
	"os/exec"

	"github.com/as/frame"
)

var winSize = image.Pt(1027, 768)

func NewFrame(disp draw.Image, origin image.Point, col frame.Colors) (*frame.Frame, *frame.Tick) {
	fr := frame.New(disp, origin,
		&frame.Option{
			Font:   frame.ParseDefaultFont(16),
			Wrap:   80,
			Scale:  image.Pt(1, 1),
			Colors: col,
		},
	)
	tick := &frame.Tick{
		Fr: fr,
		Select: frame.Select{
			Img: image.NewRGBA(fr.Bounds()),
		},
	}
	fr.Tick = tick
	return fr, tick
}

type Con struct {
	fr  *frame.Frame
	buf screen.Buffer
	kbd chan []byte
	sp  image.Point
}

func ck(err error) {
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}
}

func main() {
	selecting := false
	focused := false

	frbufs := make(map[*frame.Frame]Con)

	driver.Main(func(src screen.Screen) {
		win, _ := src.NewWindow(&screen.NewWindowOptions{winSize.X, winSize.Y})
		tx, _ := src.NewTexture(winSize)
		buf0, _ := src.NewBuffer(image.Pt(winSize.X, 40))
		buf1, _ := src.NewBuffer(image.Pt(winSize.X, winSize.Y-40))
		fr0, _ := NewFrame(buf0.RGBA(), image.Pt(10, 18), *frame.DarkGrayColors)
		fr1, tick1 := NewFrame(buf1.RGBA(), image.Pt(10, 18), *frame.GrayColors)
		kbd0, kbd1 := make(chan []byte), make(chan []byte)
		frbufs[fr0] = Con{fr0, buf0, kbd0, image.Pt(0, 0)}
		frbufs[fr1] = Con{fr1, buf1, kbd1, image.Pt(0, 40+1)}
		stdin, stdout, stderr := make(chan []byte), make(chan []byte), make(chan []byte)

		fr0.Dirty = true
		fr1.Dirty = true
		active := fr0
		selectwin := func(mouse image.Point) {
			active = fr0
			if mouse.In(fr1.Bounds().Add(image.Pt(0, fr0.Bounds().Max.Y))) {
				active = fr1
			}
		}

		if len(os.Args) < 1 {
			panic("less than 1 arg")
		}
		var cmd *exec.Cmd
		if len(os.Args) > 2 {
			cmd = exec.Command(os.Args[1], os.Args[2:]...)
		} else {
			cmd = exec.Command(os.Args[1])
		}

		cmdin, err := cmd.StdinPipe()
		ck(err)
		cmdout, err := cmd.StdoutPipe()
		ck(err)
		cmderr, err := cmd.StderrPipe()
		ck(err)

		go func(cmdout io.Reader) {
			p := make([]byte, 16484)
			for {
				n, err := cmdout.Read(p)
				if n > 0 {
					stdout <- append([]byte{}, p[:n]...)
				}
				if err != nil {
					fmt.Println("cmdout", err)
					break
				}
				fmt.Printf("cmdout: %q\n", string(p[:n]))
			}
		}(cmdout)
		go func(cmderr io.Reader) {
			p := make([]byte, 16484)
			for {
				n, err := cmderr.Read(p)
				if n > 0 {
					stderr <- append([]byte{}, p[:n]...)
				}
				if err != nil {
					fmt.Println("cmderr", err)
					break
				}
			}
		}(cmderr)

		go func() {
			for {
				select {
				case p := <-kbd0:
					//tick1.Write(p)
					stdin <- p
					win.Send(paint.Event{})
				case p := <-kbd1:
					fmt.Printf("writing %q to tick1\n", string(p))
					tick1.Write(p)
					win.Send(paint.Event{})
				}
			}
		}()

		go func() {
			for {
				select {
				case p := <-stdout:
					kbd1 <- p
				case p := <-stdin:
					println("p <- stdin", string(p))
					cmdin.Write(append(p, '\n'))
				}
			}
		}()
		go func() {
			if err := cmd.Start(); err != nil {
				panic(err)
			}
			println("program dies")
		}()

		apos := image.ZP
		for {
			switch e := win.NextEvent().(type) {
			case key.Event:
				if e.Direction != key.DirPress && e.Direction != key.DirNone {
					break
				}
				switch e.Code {
				case key.CodeRightArrow:
					if e.Modifiers != key.ModShift {
						active.Tick.P0++
					}
					active.Tick.P1++
				case key.CodeLeftArrow:
					if e.Modifiers != key.ModShift {
						active.Tick.P0--
					}
					active.Tick.P1--
				case key.CodeDeleteBackspace:
					active.Tick.Delete()
				case key.CodeReturnEnter:
					if active != fr0 {
						break
					}
					if active.Tick.P1 == active.Tick.P0 {
						for active.Tick.P0 >= 0 {
							c := active.Tick.Fr.Bytes()[active.Tick.P0]
							if c == ';' {
								active.Tick.P0++
								break
							}
							if c == '\n' {
								break
							}
							active.Tick.P0--
						}
					}
					kbd0 <- []byte(active.Tick.String())
				default:
					if e.Rune != -1 {
						active.Tick.WriteRune(e.Rune)
					}
				}
				win.Send(paint.Event{})
			case mouse.Event:
				apos = image.Pt(int(e.X), int(e.Y))
				selectwin(apos)
				if selecting {
					active.Tick.P1 = active.IndexOf(apos)
					active.Dirty = true
				}
				apos = image.Pt(apos.X, apos.Y-frbufs[active].sp.Y)
				if e.Button == mouse.ButtonLeft {
					if e.Direction == mouse.DirPress {
						active.Tick.Close()
						active.Tick.P1 = active.IndexOf(apos)
						active.Tick.SelectAt(active.Tick.P1)
						active.Tick.P0 = active.Tick.P1
						active.Dirty = true
						selecting = true
					}
					if e.Direction == mouse.DirRelease {
						selecting = false
					}
				}
				win.Send(paint.Event{})
			case size.Event, paint.Event:
				dy := 0
				for _, active := range []*frame.Frame{fr0, fr1} {
					if active.Dirty {
						active.Redraw(selecting, apos)
						drawBorder(frbufs[active].buf.RGBA(), active.Bounds().Inset(2), active.Colors.Text, image.ZP, 2)
						tx.Upload(image.Pt(0, dy), frbufs[active].buf, active.Bounds())
						win.Copy(active.Bounds().Min, tx, tx.Bounds(), screen.Over, nil)
					}
					if !focused {
						win.Copy(active.Bounds().Min, tx, tx.Bounds(), screen.Over, nil)
					}
					dy += active.Bounds().Dy()
				}
				win.Publish()
			case lifecycle.Event:
				if e.To == lifecycle.StageDead {
					return
				}

				// NT doesn't repaint the window if another window covers it
				if e.Crosses(lifecycle.StageFocused) == lifecycle.CrossOff {
					focused = false
				} else if e.Crosses(lifecycle.StageFocused) == lifecycle.CrossOn {
					focused = true
				}
			}
		}
	})
}

func drawBorder(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point, thick int) {
	draw.Draw(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+thick), src, sp, draw.Src)
	draw.Draw(dst, image.Rect(r.Min.X, r.Max.Y-thick, r.Max.X, r.Max.Y), src, sp, draw.Src)
	draw.Draw(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+thick, r.Max.Y), src, sp, draw.Src)
	draw.Draw(dst, image.Rect(r.Max.X-thick, r.Min.Y, r.Max.X, r.Max.Y), src, sp, draw.Src)
}
