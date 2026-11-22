package export

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os/exec"

	"github.com/egoist/mygo/ui"

	"mygo-uke/internal/media"
)

var errFeedEOF = errors.New("export: decoder reached end of video")

type feedFormat uint8

const (
	feedI420 feedFormat = iota
	feedRGBA
)

// feedBuffers is one more than the frames MyGo has in flight: it draws
// frame i while frame i+1 is built, so a buffer must not be reused until
// the frame before the one being built is encoded.
const feedBuffers = 4

// frameFeed decodes the source video in order and hands each frame to the
// GPU renderer. MyGo builds every frame once before drawing (to mix
// sound), then draws them, so the feed restarts the decoder when the
// requested frame moves backwards.
type frameFeed struct {
	ctx    context.Context
	cancel context.CancelFunc
	source string
	width  int
	height int

	cmd *exec.Cmd
	out io.ReadCloser
	log bytes.Buffer

	last        int
	slot        int
	currentSlot int
	format      feedFormat
	rect        image.Rectangle
	bufs        [feedBuffers][]byte
	bitmaps     [feedBuffers]*ui.Bitmap
	matrix      ui.YUVMatrix
	err         error
	eof         bool
}

func newFrameFeed(
	ctx context.Context,
	cancel context.CancelFunc,
	source string,
	width, height int,
	format feedFormat,
) *frameFeed {
	return &frameFeed{
		ctx:    ctx,
		cancel: cancel,
		source: source,
		width:  width,
		height: height,
		format: format,
		rect:   image.Rect(0, 0, width, height),
		last:   -1,
		matrix: colorMatrix(height),
	}
}

// At returns the decoded frame at index, or nil once the video ends. A
// decode error cancels the render and is reported by Err.
func (f *frameFeed) At(index int) *ui.Bitmap {
	if f.err != nil {
		return nil
	}
	if index < f.last {
		f.restart()
	}
	if f.eof {
		return f.current()
	}
	for f.last < index {
		if err := f.read(); err != nil {
			if errors.Is(err, errFeedEOF) {
				f.eof = true
				break
			}
			f.fail(err)
			return nil
		}
	}
	return f.current()
}

func (f *frameFeed) Err() error { return f.err }

// Reset restarts the decoder for another pass over the video, in format.
func (f *frameFeed) Reset(format feedFormat) {
	f.stop()
	if f.format != format {
		f.format = format
		f.bufs = [feedBuffers][]byte{}
		f.bitmaps = [feedBuffers]*ui.Bitmap{}
	}
	f.last = -1
	f.slot = 0
	f.currentSlot = 0
	f.err = nil
	f.eof = false
}

func (f *frameFeed) Close() {
	f.stop()
}

func (f *frameFeed) read() error {
	if f.out == nil {
		if err := f.start(); err != nil {
			return err
		}
	}
	buf := f.bufs[f.slot]
	if buf == nil {
		buf = make([]byte, f.frameSize())
		f.bufs[f.slot] = buf
	}
	if _, err := io.ReadFull(f.out, buf); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return errFeedEOF
		}
		return fmt.Errorf("export decoder: %w", err)
	}
	if f.format == feedRGBA {
		pixels := &image.RGBA{Pix: buf, Stride: f.width * 4, Rect: f.rect}
		if f.bitmaps[f.slot] == nil {
			f.bitmaps[f.slot] = ui.NewBitmap(pixels)
		} else {
			f.bitmaps[f.slot].Update(pixels)
		}
	} else {
		img := yuv420Image(buf, f.width, f.height, f.matrix)
		if f.bitmaps[f.slot] == nil {
			f.bitmaps[f.slot] = ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 1, 1)))
			f.bitmaps[f.slot].UpdateYUV(img)
		} else {
			f.bitmaps[f.slot].UpdateYUV(img)
		}
	}
	f.currentSlot = f.slot
	f.slot = (f.slot + 1) % len(f.bufs)
	f.last++
	return nil
}

func (f *frameFeed) current() *ui.Bitmap {
	if f.last < 0 {
		return nil
	}
	return f.bitmaps[f.currentSlot]
}

func (f *frameFeed) start() error {
	pixFmt := "yuv420p"
	format := "format=yuv420p"
	if f.format == feedRGBA {
		pixFmt, format = "rgba", "format=rgba"
	}
	filter := fmt.Sprintf(
		"scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d,fps=%d,%s",
		f.width, f.height, f.width, f.height, fps, format,
	)
	cmd := exec.CommandContext(
		f.ctx,
		media.FFmpegPath(),
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-i", f.source,
		"-vf", filter,
		"-an", "-sn", "-dn",
		"-f", "rawvideo",
		"-pix_fmt", pixFmt,
		"-",
	)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	f.log.Reset()
	cmd.Stderr = &f.log
	if err := cmd.Start(); err != nil {
		return err
	}
	f.cmd, f.out = cmd, out
	return nil
}

func (f *frameFeed) frameSize() int {
	if f.format == feedRGBA {
		return f.width * f.height * 4
	}
	return yuv420Size(f.width, f.height)
}

func (f *frameFeed) restart() {
	f.stop()
	f.last = -1
	f.eof = false
}

func (f *frameFeed) stop() {
	if f.out != nil {
		_ = f.out.Close()
		f.out = nil
	}
	if f.cmd != nil {
		if f.cmd.Process != nil {
			_ = f.cmd.Process.Kill()
		}
		_ = f.cmd.Wait()
		f.cmd = nil
	}
}

func (f *frameFeed) fail(err error) {
	if f.err == nil {
		if log := bytes.TrimSpace(f.log.Bytes()); len(log) > 0 {
			err = fmt.Errorf("%w: %s", err, log)
		}
		f.err = err
	}
	if f.cancel != nil {
		f.cancel()
	}
}

func yuv420Size(w, h int) int {
	cw, ch := (w+1)/2, (h+1)/2
	return w*h + 2*cw*ch
}

func yuv420Image(buf []byte, w, h int, matrix ui.YUVMatrix) *ui.YUVImage {
	ySize := w * h
	cw, ch := (w+1)/2, (h+1)/2
	uvSize := cw * ch
	return &ui.YUVImage{
		Format:  ui.I420,
		Width:   w,
		Height:  h,
		Planes:  [3][]byte{buf[:ySize], buf[ySize : ySize+uvSize], buf[ySize+uvSize:]},
		Strides: [3]int{w, cw, cw},
		Matrix:  matrix,
	}
}

func colorMatrix(height int) ui.YUVMatrix {
	if height >= 720 {
		return ui.BT709
	}
	return ui.BT601
}
