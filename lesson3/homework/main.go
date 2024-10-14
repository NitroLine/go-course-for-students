package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Options struct {
	From      string
	To        string
	Offset    int64
	Limit     uint
	BlockSize uint
	Conv      string
}

type ConvOption string

const (
	UpperCase  ConvOption = "upper_case"
	LowerCase             = "lower_case"
	TrimSpaces            = "trim_spaces"
)

var ConvMapper = map[string]ConvOption{
	"upper_case":  UpperCase,
	"lower_case":  LowerCase,
	"trim_spaces": TrimSpaces,
}

func (o *Options) ParseConv() ([]ConvOption, error) {
	result := make([]ConvOption, 0, 2)
	gotCase := false
	if o.Conv == "" {
		return result, nil
	}
	for _, s := range strings.Split(o.Conv, ",") {
		parsed, ok := ConvMapper[s]
		if !ok {
			return nil, fmt.Errorf("got unknow options while parse -conv: %s", s)
		}
		if parsed == LowerCase || parsed == UpperCase {
			if gotCase {
				return nil, fmt.Errorf("error while parse conv: can't use both upper_case and lower_case")
			}
			gotCase = true
		}
		result = append(result, parsed)
	}
	return result, nil
}

func (o *Options) Validate() error {
	if o.From != "" {
		stat, err := os.Stat(o.From)
		if err != nil {
			return err
		}
		if o.Offset > stat.Size() {
			return fmt.Errorf("provided offset is bigger then file size : %d > %d", o.Offset, stat.Size())
		}
	}
	if o.To != "" {
		_, err := os.Stat(o.To)
		if !os.IsNotExist(err) {
			return fmt.Errorf("output %s file already exists", o.To)
		}
	}
	if o.Conv != "" {
		_, err := o.ParseConv()
		if err != nil {
			return err
		}
	}
	if o.Offset < 0 {
		return fmt.Errorf("offset can't be negative")
	}
	return nil
}

func ParseFlags() (*Options, error) {
	var opts Options
	flag.StringVar(&opts.From, "from", "", "file to read. by default - stdin")
	flag.StringVar(&opts.To, "to", "", "file to write. by default - stdout")
	flag.Int64Var(&opts.Offset, "offset", 0, "offset bytes in input file. by default - 0")
	flag.UintVar(&opts.Limit, "limit", 0, "offset bytes in input file. read all file if zero. by default - 0")
	flag.UintVar(&opts.BlockSize, "block-size", 1000, "read and write blocks bytes length. by default - 1000")
	flag.StringVar(&opts.Conv, "conv", "", "operation on text before write. available options: lower_case, upper_case, trim_spaces")
	flag.Parse()
	err := opts.Validate()
	if err != nil {
		return nil, err
	}
	return &opts, nil
}

func process(opts *Options) error {
	// init writer and reader
	parsedConv, e := opts.ParseConv()
	if e != nil {
		return e
	}
	var reader io.Reader
	if opts.From != "" {
		readFile, err := os.Open(opts.From)
		if err != nil {
			return err
		}
		defer readFile.Close()
		reader = readFile
	} else {
		reader = io.Reader(os.Stdin)
	}
	var writer io.Writer
	if opts.To != "" {
		writeFile, err := os.OpenFile(
			opts.To,
			os.O_WRONLY|os.O_TRUNC|os.O_CREATE,
			0666,
		)
		if err != nil {
			return err
		}
		defer writeFile.Close()
		writer = writeFile
	} else {
		writer = io.Writer(os.Stdout)
	}

	// main cycle
	var prevBuffer []byte
	var isSpaceEnded = false
	var totalReadBytes uint = 0
	{
		_, err := io.CopyN(io.Discard, reader, opts.Offset)
		if err != nil {
			return fmt.Errorf("apply offset failed (possible offset greater then input size): %v", err)
		}
	}
	for {
		// read block
		endFile := false
		maxReadLength := opts.BlockSize
		if opts.Limit > 0 && totalReadBytes+opts.BlockSize > opts.Limit {
			maxReadLength = opts.Limit - totalReadBytes
		}
		buffer := make([]byte, maxReadLength)

		count, err := reader.Read(buffer)
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("error while reading: %v", err)
			}
			endFile = true
		}
		// append unparsed rune bytes
		buffer = append(prevBuffer, buffer[:count]...)
		var writerBuf []byte
		// decode read bytes per rune
	SymbolIterate:
		for len(buffer) > 0 {

			r, size := utf8.DecodeRune(buffer)
			if len(buffer) < 3 && (size <= 0 || r == utf8.RuneError) {
				break
			}

			// handle conv operations
			for _, conv := range parsedConv {
				if conv == UpperCase {
					r = unicode.To(unicode.UpperCase, r)
				}
				if conv == LowerCase {
					r = unicode.To(unicode.LowerCase, r)
				}
				if conv == TrimSpaces {
					if unicode.IsSpace(r) {
						if !isSpaceEnded {
							buffer = buffer[size:]
							continue SymbolIterate
						}
					} else {
						isSpaceEnded = true
					}
				}
			}
			var newWriteBuf []byte
			if r == utf8.RuneError {
				newWriteBuf = append(newWriteBuf, buffer[:size]...)
			} else {
				newWriteBuf = utf8.AppendRune(writerBuf, r)
			}

			if (uint)(len(newWriteBuf)) > opts.BlockSize {
				break
			}
			writerBuf = newWriteBuf
			buffer = buffer[size:]
		}
		// save unparsed rune bytes
		prevBuffer = buffer
		// write to output
		if isSpaceEnded {
			writerBuf = bytes.TrimRightFunc(writerBuf, unicode.IsSpace)
		}

		_, err = writer.Write(writerBuf)
		if err != nil {
			return err
		}
		totalReadBytes += (uint)(count)
		if endFile || (opts.Limit > 0 && totalReadBytes >= opts.Limit) {
			_, err = writer.Write(prevBuffer)
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func main() {
	opts, err := ParseFlags()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "can not parse flags:", err)
		os.Exit(1)
	}
	err = process(opts)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "error while processing:", err)
		os.Exit(1)
	}
}
