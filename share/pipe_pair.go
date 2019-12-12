package chshare

import (
	"io"
	"os"
)

type PipePair struct {
	*Logger
	reader io.ReadCloser
	writer io.WriteCloser
}

func NewStdioPipePair(logger *Logger) *PipePair {
	return NewPipePair(logger, os.Stdin, os.Stdout)
}

func NewPipePair(logger *Logger, reader io.ReadCloser, writer io.WriteCloser) *PipePair {
	return &PipePair{
		Logger: logger.Fork("PipePair(%s, %s)", reader, writer),
		reader: reader,
		writer: writer,
	}
}

func (pp *PipePair) Read(p []byte) (n int, err error) {
	// pp.Logger.Debugf("Begin Read")
	n, err = pp.reader.Read(p)
	// pp.Logger.Debugf("End Read -> n=%d, err=%s", n, err)
	return n, err
}

func (pp *PipePair) Write(p []byte) (n int, err error) {
	// pp.Logger.Debugf("Begin Write")
	n, err = pp.writer.Write(p)
	// pp.Logger.Debugf("End Write -> n=%d, err=%s", n, err)
	return n, err
}

func (pp *PipePair) Close() error {
	pp.Logger.Debugf("Closing", pp.writer)
	// pp.Logger.Debugf("Begin Close of writer %s", pp.writer)
	err := pp.writer.Close()
	// pp.Logger.Debugf("End writer Close -> err=%s", err)
	// pp.Logger.Debugf("Begin Close of reader %s", pp.reader)
	rerr := pp.reader.Close()
	// pp.Logger.Debugf("End reader Close -> err=%s", rerr)
	if err == nil {
		err = rerr
	}
	// pp.Logger.Debugf("End Close -> err=%s", err)
	return err
}
