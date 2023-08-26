package rbdconv

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"

	"github.com/ulikunitz/xz"
)

type Writer struct {
	w         io.WriteCloser
	blocksize uint64
	order     uint64 // default: order 22 (4 MiB objects)
	stripe    uint64
	buf       []byte
	offset    uint64
	size      uint64
	lk        sync.Mutex
}

func NewWriter(w io.Writer, size uint64) (*Writer, error) {
	cw, err := xz.NewWriter(w)
	if err != nil {
		return nil, err
	}

	res := &Writer{
		w:         cw,
		blocksize: 4096,
		order:     22,
		size:      size,
	}
	res.stripe = 1 << res.order

	// check for sane blocksize
	if res.size&(res.blocksize-1) != 0 {
		return nil, fmt.Errorf("filesize %d is not blocksize aligned (blocksize=%d)", res.size, res.blocksize)
	}

	err = res.writeHeader()
	if err != nil {
		return nil, err
	}

	return res, nil
}

func RawToRbd(w io.Writer, r io.Reader) error {
	// we need to find out r size
	if s, ok := r.(io.Seeker); ok {
		sz, err := s.Seek(0, io.SeekEnd)
		if err != nil {
			return fmt.Errorf("failed to seek input: %w", err)
		}
		_, err = s.Seek(0, io.SeekStart)
		if err != nil {
			return err
		}

		wr, err := NewWriter(w, uint64(sz))
		if err != nil {
			return err
		}
		_, err = io.Copy(wr, r)
		if err != nil {
			return err
		}
		return wr.Close()
	}

	return fmt.Errorf("could not determine size of input type %T", r)
}

func (w *Writer) Write(b []byte) (int, error) {
	w.lk.Lock()
	defer w.lk.Unlock()

	w.buf = append(w.buf, b...)
	if uint64(len(w.buf)) < w.stripe {
		return len(b), nil
	}

	for uint64(len(w.buf)) >= w.stripe {
		s := w.buf[:w.stripe]
		w.buf = w.buf[w.stripe:]

		offt := w.offset
		w.offset += w.stripe

		err := w.writeBuffer(s, offt)
		if err != nil {
			return 0, err
		}
	}
	return len(b), nil
}

func (w *Writer) Close() error {
	err := w.flush()
	if err != nil {
		return err
	}
	return w.writeFooter()
}

func (w *Writer) writeBuffer(buf []byte, offt uint64) error {
	// remove trailing nil characters
	bufLn := len(buf)
	for bufLn > 0 && buf[bufLn-1] == 0 {
		bufLn -= 1
	}
	if bufLn == 0 {
		// do not do any write if only zeroes
		return nil
	}

	pad := bufLn & 0xfff
	if pad != 0 {
		// make sure bufLn is aligned to 0x1000 (blocksize?)
		bufLn += 0x1000 - pad
	}
	buf = buf[:bufLn]

	headerBuf := make([]byte, 24)
	binary.LittleEndian.PutUint64(headerBuf[:8], uint64(bufLn)+8+8)
	binary.LittleEndian.PutUint64(headerBuf[8:16], offt)
	binary.LittleEndian.PutUint64(headerBuf[16:], uint64(bufLn))
	err := w.writeRecord('w', headerBuf)
	if err != nil {
		return err
	}
	_, err = w.w.Write(buf)
	return err
}

func (w *Writer) flush() error {
	if len(w.buf) == 0 {
		w.buf = nil // just in case
		return nil
	}

	s := w.buf
	w.buf = nil

	offt := w.offset
	w.offset += uint64(len(s))

	return w.writeBuffer(s, offt)
}

func (w *Writer) writeFooter() error {
	_, err := w.w.Write([]byte{'e'}) // "end"
	return err
}

func (w *Writer) writeByte(b byte) error {
	_, err := w.w.Write([]byte{b})
	return err
}

func (w *Writer) writeUint64(v uint64) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, v)
	_, err := w.w.Write(buf)
	return err
}

func (w *Writer) writeRecord(typ byte, data []byte) error {
	recHead := make([]byte, 9)
	recHead[0] = typ
	binary.LittleEndian.PutUint64(recHead[1:], uint64(len(data)))

	_, err := w.w.Write(recHead)
	if err != nil {
		return err
	}
	_, err = w.w.Write(data)
	return err
}

func (w *Writer) writeUint64Rec(typ byte, v uint64) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, v)
	return w.writeRecord(typ, buf)
}

func (w *Writer) writeHeader() error {
	_, err := w.w.Write([]byte("rbd image v2\n"))
	if err != nil {
		return err
	}
	err = w.writeUint64Rec('O', w.order)
	if err != nil {
		return err
	}
	err = w.writeUint64Rec('T', 61) // ??? probably features: layering, exclusive-lock, object-map, fast-diff, deep-flatten
	if err != nil {
		return err
	}
	err = w.writeUint64Rec('U', w.stripe) // stripe unit
	if err != nil {
		return err
	}
	err = w.writeUint64Rec('C', 1) // stripe count
	if err != nil {
		return err
	}
	err = w.writeByte('E')
	if err != nil {
		return err
	}

	// not documented
	// static const std::string RBD_IMAGE_DIFFS_BANNER_V2 ("rbd image diffs v2\n");
	_, err = w.w.Write([]byte("rbd image diffs v2\n"))
	if err != nil {
		return err
	}
	err = w.writeUint64(1)
	if err != nil {
		return err
	}

	// https://github.com/ceph/ceph/blob/master/doc/dev/rbd-diff.rst
	_, err = w.w.Write([]byte("rbd diff v2\n"))
	if err != nil {
		return err
	}
	err = w.writeUint64Rec('s', w.size)
	return err
}
