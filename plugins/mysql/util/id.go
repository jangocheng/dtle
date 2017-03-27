package util

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ngaut/log"
)

//http://sumory.com/2014/02/15/id-generator/
const (
	SnsEpoch           = uint32(1490595737)
	workerIdBits       = uint32(5)
	datacenterIdBits   = uint32(5)
	maxWorkerId        = -1 ^ (-1 << workerIdBits)
	maxDatacenterId    = -1 ^ (-1 << datacenterIdBits)
	sequenceBits       = uint32(12)
	workerIdShift      = sequenceBits
	datacenterIdShift  = sequenceBits + workerIdBits
	timestampLeftShift = sequenceBits + workerIdBits + datacenterIdBits
	sequenceMask       = -1 ^ (-1 << sequenceBits)
)

type IdWorker struct {
	sequence      uint32
	lastTimestamp uint32
	workerId      uint32
	snsEpoch      uint32
	datacenterId  uint32
	mutex         sync.Mutex
}

// NewIdWorker new a snowflake id generator object.
func NewIdWorker(workerId, datacenterId uint32, snsEpoch uint32) (*IdWorker, error) {
	idWorker := &IdWorker{}
	if workerId > maxWorkerId || workerId < 0 {
		log.Errorf("worker Id can't be greater than %d or less than 0", maxWorkerId)
		return nil, errors.New(fmt.Sprintf("worker Id: %d error", workerId))
	}
	if datacenterId > maxDatacenterId || datacenterId < 0 {
		log.Errorf("datacenter Id can't be greater than %d or less than 0", maxDatacenterId)
		return nil, errors.New(fmt.Sprintf("datacenter Id: %d error", datacenterId))
	}
	idWorker.workerId = workerId
	idWorker.datacenterId = datacenterId
	idWorker.lastTimestamp = 1
	idWorker.sequence = 0
	idWorker.snsEpoch = snsEpoch
	idWorker.mutex = sync.Mutex{}
	log.Debugf("worker starting. timestamp left shift %d, datacenter id bits %d, worker id bits %d, sequence bits %d, workerid %d", timestampLeftShift, datacenterIdBits, workerIdBits, sequenceBits, workerId)
	return idWorker, nil
}

// timeGen generate a unix millisecond.
func timeGen() uint32 {
	return uint32(time.Now().UnixNano() / int64(time.Millisecond))
}

// tilNextMillis spin wait till next millisecond.
func tilNextMillis(lastTimestamp uint32) uint32 {
	timestamp := timeGen()
	for timestamp <= lastTimestamp {
		timestamp = timeGen()
	}
	return timestamp
}

// NextId get a snowflake id.
func (id *IdWorker) NextId() (uint32, error) {
	id.mutex.Lock()
	defer id.mutex.Unlock()
	timestamp := timeGen()
	if timestamp < id.lastTimestamp {
		log.Errorf("clock is moving backwards.  Rejecting requests until %d.", id.lastTimestamp)
		return 0, errors.New(fmt.Sprintf("Clock moved backwards.  Refusing to generate id for %d milliseconds", id.lastTimestamp-timestamp))
	}
	if id.lastTimestamp == timestamp {
		id.sequence = (id.sequence + 1) & sequenceMask
		if id.sequence == 0 {
			timestamp = tilNextMillis(id.lastTimestamp)
		}
	} else {
		id.sequence = 0
	}
	id.lastTimestamp = timestamp
	return (uint32(timestamp-id.snsEpoch) << timestampLeftShift) | (id.datacenterId << datacenterIdShift) | (id.workerId << workerIdShift) | id.sequence, nil
}
