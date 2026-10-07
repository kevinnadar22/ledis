package persistence

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/kevinnadar22/ledis/internal/datatypes"
)

var Magic = []byte("REDISRDB")

const Version = byte(1)

type RDB struct {
	mu     sync.Mutex
	saving bool
}

func NewRDB() *RDB {
	return &RDB{
		mu:     sync.Mutex{},
		saving: false,
	}
}

func (r *RDB) BGSave(path string, entries []datatypes.RDBEntry) error {
	if r.saving {
		return fmt.Errorf("already saving")
	}
	go func() {
		err := r.Save(path, entries)
		if err != nil {
			log.Println("error saving snapshot:", err)
		}
	}()
	return nil
}

func (r *RDB) Save(path string, entries []datatypes.RDBEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.saving {
		return fmt.Errorf("already saving")
	}
	r.saving = true
	defer func() { r.saving = false }()

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	buf := bytes.NewBuffer(nil)
	buf.Write(Magic)
	buf.WriteByte(Version)

	for _, entry := range entries {
		err := binary.Write(buf, binary.BigEndian, uint32(len(entry.Key)))
		if err != nil {
			return err
		}
		buf.WriteString(entry.Key)

		err = binary.Write(buf, binary.BigEndian, uint32(len(entry.Value)))
		if err != nil {
			return err
		}
		buf.WriteString(entry.Value)

		if entry.Expiration != nil {
			buf.WriteByte(1)
			_ = binary.Write(buf, binary.BigEndian, *entry.Expiration)
		} else {
			buf.WriteByte(0)
		}
	}

	_, err = file.Write(buf.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (r *RDB) Load(path string) ([]datatypes.RDBEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(data)

	header := buf.Next(len(Magic))
	if len(header) != len(Magic) {
		return nil, fmt.Errorf("invalid header")
	}
	if !bytes.Equal(header, Magic) {
		return nil, fmt.Errorf("invalid magic")
	}

	version, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	if version != Version {
		return nil, fmt.Errorf("invalid version: %d", version)
	}

	entries := make([]datatypes.RDBEntry, 0)

	for buf.Len() > 0 {
		var keyLen uint32
		if err := binary.Read(buf, binary.BigEndian, &keyLen); err != nil {
			return nil, err
		}
		key := string(buf.Next(int(keyLen)))

		var valueLen uint32
		if err := binary.Read(buf, binary.BigEndian, &valueLen); err != nil {
			return nil, err
		}
		value := string(buf.Next(int(valueLen)))

		hasExp, err := buf.ReadByte()
		if err != nil {
			return nil, err
		}

		var expiration *int64
		if hasExp == 1 {
			var exp int64
			if err := binary.Read(buf, binary.BigEndian, &exp); err != nil {
				return nil, err
			}
			expiration = &exp
		}

		entries = append(entries, datatypes.RDBEntry{Key: key, Value: value, Expiration: expiration})
	}

	return entries, nil
}

func (r *RDB) WriteHeader(file *os.File) error {
	_, err := file.Write(Magic)
	if err != nil {
		return err
	}
	_, err = file.Write([]byte{Version})
	if err != nil {
		return err
	}
	return nil
}

func (r *RDB) Create(path string) error {
	// create only if file does not exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()
		return r.WriteHeader(file)
	}
	return nil
}
