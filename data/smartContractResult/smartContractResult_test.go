package smartContractResult_test

import (
	"math/big"
	"testing"

	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/smartContractResult"
	"github.com/stretchr/testify/assert"
)

func TestSmartContractResult_SettersAndGetters(t *testing.T) {
	t.Parallel()

	nonce := uint64(5)
	gasLimit := uint64(10)
	scr := smartContractResult.SmartContractResult{
		Nonce:    nonce,
		GasLimit: gasLimit,
		Value:    make(map[string]int64),
	}

	rcvAddr := []byte("rcv address")
	sndAddr := []byte("snd address")
	value := big.NewInt(37)
	data := []byte("unStake")

	scr.SetRcvAddr(rcvAddr)
	scr.SetSndAddr(sndAddr)
	scr.SetValue("", value)
	scr.SetData(data)

	assert.Equal(t, sndAddr, scr.GetSndAddr())
	assert.Equal(t, rcvAddr, scr.GetRcvAddr())
	assert.Equal(t, value.Int64(), scr.GetValue()[""])
	assert.Equal(t, data, scr.GetData()[0])
	assert.Equal(t, gasLimit, scr.GetGasLimit())
	assert.Equal(t, nonce, scr.GetNonce())
}

func TestTrimSlicePtr(t *testing.T) {
	t.Parallel()

	scrSlice := make([]*smartContractResult.SmartContractResult, 0, 5)
	scr1 := &smartContractResult.SmartContractResult{Nonce: 3}
	scr2 := &smartContractResult.SmartContractResult{Nonce: 5}

	scrSlice = append(scrSlice, scr1)
	scrSlice = append(scrSlice, scr2)

	assert.Equal(t, 2, len(scrSlice))
	assert.Equal(t, 5, cap(scrSlice))

	scrSlice = smartContractResult.TrimSlicePtr(scrSlice)

	assert.Equal(t, 2, len(scrSlice))
	assert.Equal(t, 2, len(scrSlice))
}

func TestSmartContractResult_CheckIntegrityShouldWork(t *testing.T) {
	t.Parallel()

	scr := &smartContractResult.SmartContractResult{
		Nonce:      1,
		Value:      map[string]int64{"KLV": 10},
		GasLimit:   10,
		SCData:     []byte("data"),
		RcvAddr:    []byte("rcv-address"),
		SndAddr:    []byte("snd-address"),
		PrevTxHash: []byte("prev-hash"),
	}

	err := scr.CheckIntegrity()
	assert.Nil(t, err)
}

func TestSmartContractResult_CheckIntegrityShouldErr(t *testing.T) {
	t.Parallel()

	scr := &smartContractResult.SmartContractResult{
		Nonce:  1,
		SCData: []byte("data"),
		Value:  make(map[string]int64),
	}

	err := scr.CheckIntegrity()
	assert.Equal(t, data.ErrNilRcvAddr, err)

	scr.RcvAddr = []byte("rcv-address")

	err = scr.CheckIntegrity()
	assert.Equal(t, data.ErrNilSndAddr, err)

	scr.SndAddr = []byte("snd-address")

	err = scr.CheckIntegrity()
	assert.Equal(t, data.ErrNilTxHash, err)

	scr.Value["klv"] = -1

	err = scr.CheckIntegrity()
	assert.Equal(t, data.ErrNegativeValue, err)

	scr.Value["klv"] = 10

	err = scr.CheckIntegrity()
	assert.Equal(t, data.ErrNilTxHash, err)
}
