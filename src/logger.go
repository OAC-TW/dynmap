package webmap

import (
	"fmt"
	"log"
	"io"
	"os"
	"sync"
	"time"
)

var Verbosity = 3

func Vf(level int, format string, v ...interface{}) {
	if level <= Verbosity {
		logger.Printf(format, v...)
	}
}
func Vln(level int, v ...interface{}) {
	if level <= Verbosity {
		logger.Println(v...)
	}
}

func SetFileOutput(filePath string) error {
	return logger.SetFileOutput(filePath)
}

var logger *Logger // stderr

type Logger struct {
	mx    sync.Mutex
	flag int
	outStd io.Writer // for stderr or stdout, print only
	buf []byte
	out io.WriteCloser // file fd will do rotate

	filePath string // "xxx/access-%v.log", "%v" replace by date "2020-06-01"
	daily bool
}

func NewLogger(out io.Writer, flag int) *Logger {
	lg := &Logger{}
	lg.flag = flag
	lg.outStd = out
	lg.daily = false
	return lg
}

func NewLoggerFile(outprint io.Writer, filePath string, flag int) (*Logger, error) {
	lg := &Logger{}
	lg.flag = flag
	lg.daily = true
	lg.filePath = filePath

	if outprint != nil {
		lg.outStd = outprint
	}

	fd, err := lg.rotateLog()
	if err != nil {
		return nil, err
	}
	lg.out = fd

	go lg.rotateDaily()
	return lg, nil
}

func init() {
	logger = NewLogger(os.Stderr, log.LstdFlags|log.Lmicroseconds)
}

func (lg *Logger) SetPrintOutput(w io.Writer) {
	lg.mx.Lock()
	defer lg.mx.Unlock()
	lg.outStd = w
}

func (lg *Logger) SetFileOutput(filePath string) error {
	lg.mx.Lock()
	defer lg.mx.Unlock()
	lg.filePath = filePath
	fd, err := lg.rotateLog()
	if err != nil {
		return err
	}
	lg.out = fd
	if !lg.daily {
		lg.daily = true
		go lg.rotateDaily()
	}
	return nil
}

func (lg *Logger) Printf(format string, v ...interface{}) {
	lg.Output(fmt.Sprintf(format, v...))
}
func (lg *Logger) Println(v ...interface{}) {
	lg.Output(fmt.Sprintln(v...))
}

func (lg *Logger) formatHeader(buf *[]byte, t time.Time, file string, line int) {
	if lg.flag&log.LUTC != 0 {
		t = t.UTC()
	}
	if lg.flag&log.Ldate != 0 {
		year, month, day := t.Date()
		itoa(buf, year, 4)
		*buf = append(*buf, '/')
		itoa(buf, int(month), 2)
		*buf = append(*buf, '/')
		itoa(buf, day, 2)
		*buf = append(*buf, ' ')
	}
	if lg.flag&(log.Ltime|log.Lmicroseconds) != 0 {
		hour, min, sec := t.Clock()
		itoa(buf, hour, 2)
		*buf = append(*buf, ':')
		itoa(buf, min, 2)
		*buf = append(*buf, ':')
		itoa(buf, sec, 2)
		if lg.flag&log.Lmicroseconds != 0 {
			*buf = append(*buf, '.')
			itoa(buf, t.Nanosecond()/1e3, 6)
		}
		*buf = append(*buf, ' ')
	}
}

func (lg *Logger) Output(s string) error {
	now := time.Now() // get this early.
	var file string
	var line int
	var err error
	lg.mx.Lock()
	defer lg.mx.Unlock()
	lg.buf = lg.buf[:0]
	lg.formatHeader(&lg.buf, now, file, line)
	lg.buf = append(lg.buf, s...)
	if len(s) == 0 || s[len(s)-1] != '\n' {
		lg.buf = append(lg.buf, '\n')
	}

	if lg.outStd != nil {
		_, err = lg.outStd.Write(lg.buf)
		if err != nil {
			return err
		}
	}
	if lg.out != nil {
		_, err = lg.out.Write(lg.buf)
	}
	return err
}

func (lg *Logger) rotateDaily() {
	for {
		if !lg.daily {
			break
		}

		t := time.Now()
		if lg.flag&log.LUTC != 0 {
			t = t.UTC()
		}
		h, m, s := t.Clock()
		d := time.Duration((24-h)*3600-m*60-1*s) * time.Second
		tmr := time.NewTimer(d)
		select {
		case <-tmr.C:
			f, err := lg.rotateLog()
			if err != nil {
				_ = fmt.Errorf("Failed to rotate log daily: %v", err)
			}
			lg.out = f
		}
	}
}

func (lg *Logger) rotateLog() (f *os.File, err error) {
	// close log file
	if lg.out != nil {
		// ignore err
		lg.out.Close()
	}

	t := time.Now()
	if lg.flag&log.LUTC != 0 {
		t = t.UTC()
	}
	year, month, day := t.Date()
	dateStr := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	fp := fmt.Sprintf(lg.filePath, dateStr)
	f, err = os.OpenFile(fp, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0640)
	if err != nil {
		return nil, err
	}

	return f, nil
}

// from src/log/log.go
// Cheap integer to fixed-width decimal ASCII. Give a negative width to avoid zero-padding.
func itoa(buf *[]byte, i int, wid int) {
	// Assemble decimal in reverse order.
	var b [20]byte
	bp := len(b) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	b[bp] = byte('0' + i)
	*buf = append(*buf, b[bp:]...)
}

