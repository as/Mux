package main

import (
	//	"github.com/as/clip"

	"bufio"
//	"fmt"
	"image"
	"image/draw"
	"io"
	"os"
	"os/exec"
	//	"image/color"
	"time"

	"github.com/as/frame/tag"
	"golang.org/x/exp/shiny/driver"
	"golang.org/x/exp/shiny/screen"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/mobile/event/key"
	"golang.org/x/mobile/event/lifecycle"
	"golang.org/x/mobile/event/mouse"
	"golang.org/x/mobile/event/paint"
	"golang.org/x/mobile/event/size"

	"bytes"

	"github.com/as/frame"
)

var winSize = image.Pt(800, 1440)

type Con struct {
	fr  *frame.Frame
	kbd chan []byte
	sp  image.Point
}

func ck(err error) {
	if err != nil {
		println(err.Error())
		os.Exit(1)
	}
}

type InsertEvent struct {
	f      tag.File
	q0, q1 int64
	p      []byte
}
type StdinEvent struct {
	in io.WriteCloser
	p  []byte
}

func mkfont(size int) frame.Font {
	return frame.NewTTF(gomono.TTF, size)
}
func main() {
	focused := false
	/*
		Gray := *frame.DarkGrayColors
		Text := frame.DefaultColors.Text
		Back := frame.DefaultColors.Back
	*/

	driver.Main(func(src screen.Screen) {
		wind, _ := src.NewWindow(&screen.NewWindowOptions{winSize.X, winSize.Y, "cmd"})
		t := tag.NewTag(src, wind, mkfont(11), image.ZP, winSize, image.Pt(10, 5), frame.Acme)
		active := t.W
		var qcmd int64
		selectwin := func(mouse image.Point) {
			active = t.W
			if mouse.In(t.Wtag.Loc()) {
				active = t.Wtag
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
			p := make([]byte, 1024*1024)
			for {
				time.Sleep(time.Second / 30)
				n, err := cmdout.Read(p)
				if n > 0 {
					p := append([]byte{}, p[:n]...)
					wind.Send(InsertEvent{f: t.W, q1: qcmd, p: p})
				}
				if err != nil {
					break
				}
			}
		}(bufio.NewReader(cmdout))
		go func(cmderr io.Reader) {
			p := make([]byte, 1024*1024)
			for {
				time.Sleep(time.Second / 30)
				n, err := cmderr.Read(p)
				if n > 0 {
					p := append([]byte{}, p[:n]...)
					wind.Send(InsertEvent{f: t.W, q1: qcmd, p: p})
				}
				if err != nil {
					break
				}
			}
		}(bufio.NewReader(cmderr))

		go func() {
			if err := cmd.Start(); err != nil {
				panic(err)
			}
			println("program dies")
		}()
		selectwin = selectwin
		qcmd = int64(len(t.W.Bytes()))
		for {
			switch e := wind.NextEvent().(type) {
			case StdinEvent:
				io.Copy(e.in, bytes.NewReader(e.p))
			case InsertEvent:
				e.f.Insert(e.p, qcmd)
				qcmd += int64(len(e.p))
				if e.f.Dirty() {
					wind.Send(paint.Event{})
				}
			case key.Event:
				if e.Direction == 2 {
					continue
				}
				if e.Rune == '\r' {
					e.Rune = '\n'
				}
				l0 := int64(len(active.Bytes()))
				t.Handle(t.W, e)
				_, q1 := active.Dot()
				if q1 > qcmd-1 {
					if e.Rune == '\n' {
						p := append([]byte{}, t.W.Bytes()[qcmd:q1]...)
						qcmd = q1
						wind.Send(StdinEvent{cmdin, p})
					}
				} else if l1 := int64(len(active.Bytes())); l0 > l1 {
					qcmd -= (l0 - l1)
				} else {
					qcmd++
				}
			case mouse.Event:
				//pt := image.Pt(int(e.X), int(e.Y))
				//selectwin(pt)
				e.X -= float32(t.W.Loc().Min.X)
				e.Y -= float32(t.W.Loc().Min.Y)
				t.Handle(active, e)
			case size.Event:
				winSize.X = e.WidthPx
				winSize.Y = e.HeightPx
				t.Resize(winSize)
			case paint.Event:
				//paintcnt++
				//fmt.Printf("%08d %#v\n", paintcnt, e)
				//				fr := t.W.Frame; fr.Paint(fr.PointOf(2), fr.PointOf(qcmd-t.W.Org), image.NewUniform(color.RGBA{0,255,0,128}))
				t.Upload(wind)
				wind.Publish()
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

var paintcnt = 0

func drawBorder(dst draw.Image, r image.Rectangle, src image.Image, sp image.Point, thick int) {
	draw.Draw(dst, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+thick), src, sp, draw.Src)
	draw.Draw(dst, image.Rect(r.Min.X, r.Max.Y-thick, r.Max.X, r.Max.Y), src, sp, draw.Src)
	draw.Draw(dst, image.Rect(r.Min.X, r.Min.Y, r.Min.X+thick, r.Max.Y), src, sp, draw.Src)
	draw.Draw(dst, image.Rect(r.Max.X-thick, r.Min.Y, r.Max.X, r.Max.Y), src, sp, draw.Src)
}
