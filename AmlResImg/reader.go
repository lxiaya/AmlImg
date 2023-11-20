package AmlResImg

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/lxiaya/AmlImg/AmlCRC"
)

type ImageReader struct {
	file *os.File

	Header *Header
	Items  []*Item

	remain uint64
}

func NewReader(path string, check bool) (*ImageReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	header, err := Header_Unpack(file)
	if err != nil {
		return nil, err
	}

	if header.Magic != Header_Magic {
		return nil, fmt.Errorf("incorrect magic: should %08X but is %08X", Header_Magic, header.Magic)
	}

	if check {
		_, err = file.Seek(4, io.SeekStart)
		if err != nil {
			return nil, err
		}

		crc := uint32(0xffffffff)
		var buf [4096]byte
		for {
			n, err := file.Read(buf[:])
			crc = AmlCRC.AmlCRC(crc, buf[:n])
			if errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return nil, err
			}
		}

		if header.CRC != crc {
			return nil, fmt.Errorf("incorrect crc: should %08X but is %08X", header.CRC, crc)
		}
	}

	next := int64(binary.Size(Header{}))
	items := make([]*Item, header.ItemCount)
	for i := 0; i < int(header.ItemCount); i++ {
		_, err = file.Seek(next, io.SeekStart)
		if err != nil {
			return nil, err
		}

		items[i], err = Item_Unpack(file, header.Version)
		if err != nil {
			return nil, err
		}

		if items[i].Magic != Item_Magic {
			return nil, fmt.Errorf("item[%d]: incorrect magic: should %08X but is %08X", i, Header_Magic, header.Magic)
		}

		next = int64(items[i].NextItemOffset)
	}

	return &ImageReader{
		file,
		header,
		items,
		0,
	}, nil
}

func (r *ImageReader) Seek(id uint32, offset uint64) error {
	item := r.Items[id]
	_, err := r.file.Seek(int64(item.DataOffset)+int64(offset), io.SeekStart)
	r.remain = uint64(item.Size) - offset
	return err
}

func (r *ImageReader) Read(b []byte) (int, error) {
	if r.remain == 0 {
		return 0, io.EOF
	}

	size := cap(b)
	if size > int(r.remain) {
		size = int(r.remain)
	}

	n, err := r.file.Read(b[:size])
	r.remain -= uint64(n)
	return n, err
}

func (r *ImageReader) Close() {
	r.file.Close()
}
