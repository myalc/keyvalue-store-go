package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// TODO: Prevent working on huge files, consider seperating files when size gets huge
// TODO: Monitor file handlers open/close actions, do not leave open file descriptors
// TODO: when multiple instances run synch changes to other instances

// Persistance interface starts a timer, timer tick is listen by parent (ServiceX) object
// Waits current dict from the parent (ServiceX) object then persist to file system
// After a succesful persist it deletes the old files
// On startup it check file system for a previosly persisted dict
// Checks the hash of current and previosly persisted dictionary and decides to persist or not
type Persistance interface {
	StartTicker(interval int)
	Persist(dict *map[string]string) string
	DeleteOldFiles(newFilename string)
	RestoreFromPersistance() (map[string]string, error)
	CheckIfHashIsSame(buf []byte) (bool, uint32)
}

// FSPersistance holds ticker, persistanceChan to receive current dict from ServiceX, and latest hash of persisted dict
type FSPersistance struct {
	// ticker will be listened by Service object
	// when timer ticks Service will send current dict to Persistance via persistanceChan
	ticker          *time.Ticker
	persistanceChan chan map[string]string
	latesHash       uint32
}

// NewPersistance creates a new FSPersistance, initializes channel and starts the timer, and a go routine listens dict from ServiceX
func NewPersistance(interval int) *FSPersistance {
	var p FSPersistance
	p.persistanceChan = make(chan map[string]string, 10) // it is not necessary to make it buffered.
	// but when Persist takes longer than interval; making it buffered will prevent blocking main routine
	p.StartTicker(interval)
	return &p
}

// StartTicker starts the timer and listener
func (p *FSPersistance) StartTicker(interval int) {
	p.ticker = time.NewTicker(time.Duration(interval) * 1000 * time.Millisecond)
	// When Service routine sends data over channel; following routine will get dict and persist to file system
	go func() {
		for {
			receivedDict := <-p.persistanceChan
			log.Printf("DEBUG Reecived dict at %v. dict.len:%v dict:%v", time.Now(), len(receivedDict), receivedDict)
			p.Persist(&receivedDict)
			// no need give ack
		}
	}()
}

// Persist writes current data to file system
// It does not check the size of dict to able to write data after delete all operation
// Compares hash values of current and previosly persisted dict and decides to persist or not
func (p *FSPersistance) Persist(dict *map[string]string) string {
	/*if (len(*dict)) == 0 {
		return "" // additional check
	}*/

	jsonStr, _err2 := json.Marshal(*dict)
	if _err2 != nil {
		log.Printf("ERROR json.Marshal failed. err:%v\r\n", _err2.Error())
		return ""
	}
	same, hash := p.CheckIfHashIsSame(jsonStr)
	if same {
		_ = hash
		return ""
	}

	filename := filepath.Join(os.TempDir(), "GOAPP-"+strconv.FormatInt(time.Now().Unix(), 10)+".json")
	var dest, _err = os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0660)
	if _err != nil {
		log.Printf("ERROR os.Create: %v\r\n", _err.Error())
		return ""
	}
	defer dest.Close()

	n, _err3 := fmt.Fprintf(dest, "%s\n", jsonStr)
	if _err3 != nil {
		log.Printf("ERROR Write file failed. err:%v\r\n", _err3.Error())
		return ""
	}

	p.latesHash = hash // update with new value
	log.Printf("INFO dict (%v) persisted into %v", n, filename)
	p.DeleteOldFiles(filename)
	return filename
}

// DeleteOldFiles deletes old files after a successful persist
func (p *FSPersistance) DeleteOldFiles(newFilename string) {
	dir, _err := os.Open(os.TempDir())
	if _err != nil {
		log.Printf("WARNING Cannot open tmp directory %v. err:%v\r\n", os.TempDir(), _err)
		return
	}
	files, _err2 := dir.Readdir(0)
	if _err2 != nil {
		log.Printf("WARNING Cannot read tmp directory %v. err:%v\r\n", os.TempDir(), _err2)
		return
	}
	defer dir.Close()

	// Delete all GOAPP-*.json files except the new file
	for _, v := range files {
		filename := filepath.Join(os.TempDir(), v.Name())
		if filename != newFilename && strings.HasPrefix(v.Name(), "GOAPP") && strings.HasSuffix(v.Name(), ".json") && !v.IsDir() {
			_err3 := os.Remove(filename)
			if _err3 != nil {
				log.Printf("ERROR Cannot delete file %v. err:%v\r\n", filename, _err3)
			} else {
				log.Printf("INFO Deleted file %v.\r\n", filename)
			}
		}
	}
}

// RestoreFromPersistance checks the file system for previosly persisted dict
func (p *FSPersistance) RestoreFromPersistance() (map[string]string, error) {
	dir, _err := os.Open(os.TempDir())
	if _err != nil {
		log.Printf("WARNING Cannot open tmp directory %v. err:%v\r\n", os.TempDir(), _err)
		return nil, _err
	}
	files, _err2 := dir.Readdir(0)
	if _err2 != nil {
		log.Printf("WARNING Cannot read tmp directory %v. err:%v\r\n", os.TempDir(), _err2)
		return nil, _err2
	}
	defer dir.Close()

	// select the latest file (in case multiple files found)
	var latestFile string
	var latestTs uint64 = 0
	for _, v := range files {
		if strings.HasPrefix(v.Name(), "GOAPP") && strings.HasSuffix(v.Name(), ".json") && !v.IsDir() {
			_ts := strings.Replace(v.Name(), "GOAPP-", "", 6)
			ts := strings.Replace(_ts, ".json", "", 4)
			i, _ := strconv.ParseUint(ts, 10, 64)
			if i > latestTs {
				latestTs = i
				latestFile = v.Name()
			}
		}
	}

	if latestTs == 0 {
		log.Printf("INFO Cannot find any GOAPP-*.json file in %v directory.", os.TempDir())
		return nil, errors.New(fmt.Sprintf("INFO Cannot fing any GOAPP-*.json file in %v directory.", os.TempDir()))
	}

	// Read the file
	filename := filepath.Join(os.TempDir(), latestFile)
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Printf("ERROR Cannot read %v. err:%v", filename, err)
		return nil, err
	}

	// create dict
	var dict map[string]string
	err = json.Unmarshal([]byte(buf), &dict)
	if err != nil {
		log.Printf("ERROR Cannot unmarshal buffer %v. err:%v", buf, err)
		return nil, err
	}

	log.Printf("DEBUG restored dict.size: %v, dict:%v", len(dict), dict)
	return dict, nil
}

// CheckIfHashIsSame compares hash values
func (p *FSPersistance) CheckIfHashIsSame(buf []byte) (bool, uint32) {
	h := fnv.New32a()
	h.Write(buf)
	hash := h.Sum32()
	log.Printf("DEBUG Hash values: Current dict: %v. Previosly persisted dict: %v\r\n", hash, p.latesHash)
	if p.latesHash == hash {
		return true, hash
	} else {
		return false, hash
	}
}
