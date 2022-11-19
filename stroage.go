package tiny_bitcask

import (
	"errors"
	"fmt"
	"os"
)

var (
	readMissDataErr  = errors.New("miss data during read")
	writeMissDataErr = errors.New("miss data during write")
	crcErr           = errors.New("crc error")
)

const (
	fileSuffix = "dat"
	B          = 1
	KB         = 1024 * B
	MB         = 1024 * KB
	GB         = 1024 * MB
)

type Storage struct {
	dir      string
	fileSize int64
	af       *ActiveFile
	fds      map[int]*os.File
}

func NewStorage(dir string) (s *Storage, err error) {
	err = os.Mkdir(dir, os.ModePerm)
	if err != nil {
		return nil, err
	}
	path := s.getPath()
	fd, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		return nil, err
	}
	s = &Storage{
		dir: dir,
		af: &ActiveFile{
			fid: 0,
			f:   fd,
			off: 0,
		},
		fds: map[int]*os.File{0: fd},
	}
	return s, nil
}

type ActiveFile struct {
	fid int
	f   *os.File
	off int64
}

func (s *Storage) readEntry(fid int, off int64) (e *Entry, err error) {
	buf := make([]byte, MetaSize)
	err = s.readAt(fid, off, buf)
	if err != nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	e = NewEntry()
	e.DecodeMeta(buf)
	off += MetaSize
	payloadSize := e.meta.keySize + e.meta.valueSize
	payload := make([]byte, payloadSize)
	err = s.readAt(fid, off, payload)
	if err != nil {
		return nil, err
	}
	err = e.DecodePayload(payload)
	if err != nil {
		return nil, err
	}
	crc := e.getCrc(buf)
	if e.meta.crc != crc {
		return nil, crcErr
	}
	return e, nil
}

func (s *Storage) readAt(fid int, off int64, bytes []byte) (err error) {
	if fd := s.fds[fid]; fd != nil {
		n, err := fd.ReadAt(bytes, off)
		if n < len(bytes) {
			return readMissDataErr
		}
		return err
	}
	path := fmt.Sprintf("%s/%d", s.dir, fid)
	fd, err := os.OpenFile(path, os.O_RDWR, os.ModePerm)
	s.fds[fid] = fd
	n, err := fd.ReadAt(bytes, off)
	if n < len(bytes) {
		return readMissDataErr
	}
	return err
}

func (s *Storage) writeAt(bytes []byte) (i *Index, err error) {
	err = s.af.writeAt(bytes)
	if err != nil {
		return nil, err
	}
	i = &Index{
		fid: s.af.fid,
		off: s.af.off,
	}
	s.af.off += int64(len(bytes))
	if s.af.off > s.fileSize {
		err := s.rotate()
		if err != nil {
			return nil, err
		}
	}
	return i, nil
}

func (af *ActiveFile) writeAt(bytes []byte) error {
	n, err := af.f.WriteAt(bytes, af.off)
	if n < len(bytes) {
		return writeMissDataErr
	}
	return err
}

func (s *Storage) rotate() error {
	af := &ActiveFile{
		fid: s.af.fid + 1,
		off: 0,
	}
	fd, err := os.OpenFile(s.getPath(), os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		return err
	}
	af.f = fd
	s.fds[af.fid] = fd
	s.af = af
	return nil
}

func (s *Storage) getPath() string {
	path := fmt.Sprintf("%s/%d.%s", s.dir, s.af.fid, fileSuffix)
	return path
}