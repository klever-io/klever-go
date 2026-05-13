package broadcast

import (
	"bytes"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/klever-io/klever-go/common"
	"github.com/klever-io/klever-go/core"
	"github.com/klever-io/klever-go/core/consensus"
	"github.com/klever-io/klever-go/core/process"
	"github.com/klever-io/klever-go/data"
	"github.com/klever-io/klever-go/data/block"
	"github.com/klever-io/klever-go/storage"
	"github.com/klever-io/klever-go/storage/lrucache"
	"github.com/klever-io/klever-go/tools/check"
)

const prefixHeaderAlarm = "header_"
const prefixDelayDataAlarm = "delay_"
const sizeHeadersCache = 1000 // 1000 hashes in cache

type ArgsDelayedBlockBroadcaster struct {
	InterceptorsContainer process.InterceptorsContainer
	HeadersSubscriber     consensus.HeadersPoolSubscriber
	LeaderCacheSize       uint32
	ValidatorCacheSize    uint32
	AlarmScheduler        core.TimersScheduler
}

type HeaderDataForValidator struct {
	Slot         uint64
	PrevRandSeed []byte
}

type validatorHeaderBroadcastData struct {
	headerHash       []byte
	header           data.HeaderHandler
	transactionsData [][]byte
	order            uint32
	pkBytes          []byte
}

type delayedBroadcastData struct {
	headerHash   []byte
	header       data.HeaderHandler
	transactions [][]byte
	order        uint32
	pkBytes      []byte
}

type headerDataForValidator struct {
	slot         uint64
	prevRandSeed []byte
}

var _ delayedBroadcaster = (*delayedBlockBroadcaster)(nil)

type delayedBlockBroadcaster struct {
	alarm                      core.TimersScheduler
	interceptorsContainer      process.InterceptorsContainer
	headersSubscriber          consensus.HeadersPoolSubscriber
	mutDataForBroadcast        sync.RWMutex
	mutHeadersCache            sync.RWMutex
	valBroadcastData           []*delayedBroadcastData
	maxDelayCacheSize          uint32
	maxValidatorDelayCacheSize uint32
	delayedBroadcastData       []*delayedBroadcastData
	valHeaderBroadcastData     []*validatorHeaderBroadcastData
	cacheHeaders               storage.Cacher
	broadcastHeader            func(header data.HeaderHandler) error
	broadcastTxsData           func(txData [][]byte) error
}

func NewDelayedBlockBroadcaster(args *ArgsDelayedBlockBroadcaster) (*delayedBlockBroadcaster, error) {
	if check.IfNil(args.InterceptorsContainer) {
		return nil, common.ErrNilInterceptorsContainer
	}
	if check.IfNil(args.HeadersSubscriber) {
		return nil, common.ErrNilHeadersSubscriber
	}
	if check.IfNil(args.AlarmScheduler) {
		return nil, common.ErrNilAlarmScheduler
	}

	cacheHeaders, err := lrucache.NewCache(sizeHeadersCache)
	if err != nil {
		return nil, err
	}

	delayedBroadcast := &delayedBlockBroadcaster{
		alarm:                      args.AlarmScheduler,
		interceptorsContainer:      args.InterceptorsContainer,
		headersSubscriber:          args.HeadersSubscriber,
		valHeaderBroadcastData:     make([]*validatorHeaderBroadcastData, 0),
		valBroadcastData:           make([]*delayedBroadcastData, 0),
		delayedBroadcastData:       make([]*delayedBroadcastData, 0),
		maxDelayCacheSize:          args.LeaderCacheSize,
		maxValidatorDelayCacheSize: args.ValidatorCacheSize,
		mutDataForBroadcast:        sync.RWMutex{},
		cacheHeaders:               cacheHeaders,
		mutHeadersCache:            sync.RWMutex{},
	}
	delayedBroadcast.headersSubscriber.RegisterHandler(delayedBroadcast.headerReceived)

	err = delayedBroadcast.registerHeaderInterceptorCallback(delayedBroadcast.interceptedHeader)
	if err != nil {
		return nil, err
	}

	return delayedBroadcast, nil
}

// SetLeaderData sets the data for consensus leader delayed broadcast
func (d *delayedBlockBroadcaster) SetLeaderData(broadcastData *delayedBroadcastData) error {
	if broadcastData == nil {
		return common.ErrNilParameter
	}

	log.Trace("delayedBlockBroadcaster.SetLeaderData: setting leader delay data",
		"headerHash", broadcastData.headerHash,
	)

	dataToBroadcast := make([]*delayedBroadcastData, 0)

	// save data
	d.mutDataForBroadcast.Lock()
	d.delayedBroadcastData = append(d.delayedBroadcastData, broadcastData)

	// check if the data cache has reached its limit, if so, remove the older one (FIFO)
	if len(d.delayedBroadcastData) > int(d.maxDelayCacheSize) {
		log.Debug("delayedBlockBroadcaster.SetLeaderData: leader broadcasts old data before alarm due to too much delay data",
			"headerHash", d.delayedBroadcastData[0].headerHash,
			"nbDelayedData", len(d.delayedBroadcastData),
			"maxDelayCacheSize", d.maxDelayCacheSize,
		)
		dataToBroadcast = append(dataToBroadcast, d.delayedBroadcastData[0])
		d.delayedBroadcastData = d.delayedBroadcastData[1:]
	}
	d.mutDataForBroadcast.Unlock()

	// if older info extracted, broadcast it
	if len(dataToBroadcast) > 0 {
		d.broadcastDelayedData(dataToBroadcast)
	}

	return nil
}

// SetValidatorData adds recently received data to the broadcast schedule
// and remove older data and cancel their alarms
func (d *delayedBlockBroadcaster) SetValidatorData(data *delayedBroadcastData) error {
	if data == nil {
		return common.ErrNilParameter
	}

	alarmIDsToCancel := make([]string, 0)
	log.Trace("delayedBlockBroadcaster.SetValidatorData: setting validator delay data",
		"headerHash", data.headerHash,
		"nonce", data.header.GetNonce(),
		"prevRandSeed", data.header.GetPrevRandSeed(),
	)

	d.mutDataForBroadcast.Lock()

	// Adds new data to the broadcast schedule
	d.valBroadcastData = append(d.valBroadcastData, data)

	// Remove expired data when reached max cache size
	if len(d.valBroadcastData) > int(d.maxValidatorDelayCacheSize) {
		alarmHeaderID := prefixHeaderAlarm + hex.EncodeToString(d.valBroadcastData[0].headerHash)
		alarmDelayID := prefixDelayDataAlarm + hex.EncodeToString(d.valBroadcastData[0].headerHash)
		alarmIDsToCancel = append(alarmIDsToCancel, alarmHeaderID, alarmDelayID)
		d.valBroadcastData = d.valBroadcastData[1:]
		log.Debug("delayedBlockBroadcaster.SetValidatorData: canceling old alarms (header and delay data) due to too much delay data",
			"headerHash broadcastData", d.valBroadcastData[0].headerHash,
			"headerHash", data.headerHash,
			"alarmID-header", alarmHeaderID,
			"alarmID-delay", alarmDelayID,
			"nbDelayData", len(d.valBroadcastData),
			"maxValidatorDelayCacheSize", d.maxValidatorDelayCacheSize,
		)
	}
	d.mutDataForBroadcast.Unlock()

	for _, alarmID := range alarmIDsToCancel {
		d.alarm.Cancel(alarmID)
	}

	return nil
}

// SetHeaderForValidator receives the validator header and set an alarm to re-broadcast this message
func (d *delayedBlockBroadcaster) SetHeaderForValidator(vData *validatorHeaderBroadcastData) error {
	if check.IfNil(vData.header) {
		return common.ErrNilHeader
	}
	if len(vData.headerHash) == 0 {
		return common.ErrNilHeaderHash
	}

	// skip alarm if the block was not finalized yet
	if len(vData.header.GetSignature()) == 0 {
		log.Trace("delayedBlockBroadcaster.SetHeaderForValidator: header alarm has not been set",
			"validatorConsensusOrder", vData.order,
		)
		return nil
	}

	// prevent duplicates
	_, alreadyReceived := d.cacheHeaders.Get(vData.headerHash)
	if alreadyReceived {
		return nil
	}

	// Take mutDataForBroadcast around all reads / writes of the broadcast
	// slices. Without this, the append on valHeaderBroadcastData races with
	// the locked iteration in interceptedHeader, which can corrupt the slice
	// and cause interceptedHeader to cancel the wrong header alarm —
	// validators then miss the leader header, fail to sign within the slot,
	// and the cluster stalls below quorum.
	d.mutDataForBroadcast.Lock()
	log.Trace("delayedBlockBroadcaster.SetHeaderForValidator",
		"nbDelayedBroadcastData", len(d.delayedBroadcastData),
		"nbValBroadcastData", len(d.valBroadcastData),
		"nbValHeaderBroadcastData", len(d.valHeaderBroadcastData),
	)
	duration := validatorDelayPerOrder * time.Duration(vData.order)
	d.valHeaderBroadcastData = append(d.valHeaderBroadcastData, vData)
	alarmID := prefixHeaderAlarm + hex.EncodeToString(vData.headerHash)
	d.mutDataForBroadcast.Unlock()

	// set an callback to execute after X seconds
	d.alarm.Add(d.headerAlarmExpired, duration, alarmID)
	log.Trace("delayedBlockBroadcaster.SetHeaderForValidator: header alarm has been set",
		"validatorConsensusOrder", vData.order,
		"headerHash", vData.headerHash,
		"headerSlot", vData.header.GetSlot(),
		"alarmID", alarmID,
		"duration", duration,
	)

	return nil
}

// headerAlarmExpired perform the broadcast of the header if an alarm fails
func (d *delayedBlockBroadcaster) headerAlarmExpired(alarmID string) {
	// extract header hash from alarm ID
	headerHash, err := hex.DecodeString(strings.TrimPrefix(alarmID, prefixHeaderAlarm))
	if err != nil {
		log.Error("delayedBlockBroadcaster.headerAlarmExpired", "error", err.Error(),
			"alarmID", alarmID,
		)
		return
	}

	// retrive the header data from the collection and remove it
	d.mutDataForBroadcast.Lock()
	var vHeader *validatorHeaderBroadcastData
	for i, broadcastData := range d.valHeaderBroadcastData {
		if bytes.Equal(broadcastData.headerHash, headerHash) {
			vHeader = broadcastData
			d.valHeaderBroadcastData = append(d.valHeaderBroadcastData[:i], d.valHeaderBroadcastData[i+1:]...)
			break
		}
	}
	d.mutDataForBroadcast.Unlock()

	if vHeader == nil {
		log.Debug("delayedBlockBroadcaster.headerAlarmExpired: alarm data is nil",
			"headerHash", headerHash,
			"alarmID", alarmID,
		)
		return
	}

	log.Debug("delayedBlockBroadcaster.headerAlarmExpired: validator broadcasting header",
		"headerHash", headerHash,
		"alarmID", alarmID,
	)
	// broadcast header
	err = d.broadcastHeader(vHeader.header)
	if err != nil {
		log.Warn("delayedBlockBroadcaster.headerAlarmExpired", "error", err.Error(),
			"headerHash", headerHash,
			"alarmID", alarmID,
		)
	}

	go d.broadcastBlockData(vHeader.transactionsData, common.ExtraDelayForBroadcastBlockInfo)
}

func (d *delayedBlockBroadcaster) scheduleValidatorBroadcast(headerData *headerDataForValidator) {
	type alarmParams struct {
		id       string
		duration time.Duration
	}

	alarmsToAdd := make([]alarmParams, 0)

	d.mutDataForBroadcast.RLock()
	// check if it has work to be done
	if len(d.valBroadcastData) == 0 {
		d.mutDataForBroadcast.RUnlock()
		return
	}

	log.Trace("delayedBlockBroadcaster.scheduleValidatorBroadcast",
		"slot", headerData.slot,
		"prevRandSeed", headerData.prevRandSeed,
	)

	log.Trace("delayedBlockBroadcaster.scheduleValidatorBroadcast: registered data for broadcast")
	for i := range d.valBroadcastData {
		log.Trace("delayedBlockBroadcaster.scheduleValidatorBroadcast",
			"slot", d.valBroadcastData[i].header.GetSlot(),
			"prevRandSeed", d.valBroadcastData[i].header.GetPrevRandSeed(),
		)
	}

	// match header data and validator data
	for _, broadcastData := range d.valBroadcastData {
		sameSlot := headerData.slot == broadcastData.header.GetSlot()
		samePrevRandomness := bytes.Equal(headerData.prevRandSeed, broadcastData.header.GetPrevRandSeed())
		if sameSlot && samePrevRandomness {

			// set an alarm to broadcast the block info
			duration := validatorDelayPerOrder*time.Duration(broadcastData.order) + common.ExtraDelayForBroadcastBlockInfo
			alarmID := prefixDelayDataAlarm + hex.EncodeToString(broadcastData.headerHash)

			alarmsToAdd = append(alarmsToAdd, alarmParams{
				id:       alarmID,
				duration: duration,
			})
			log.Trace("delayedBlockBroadcaster.scheduleValidatorBroadcast: scheduling delay data broadcast for notarized header",
				"headerHash", broadcastData.headerHash,
				"alarmID", alarmID,
				"slot", headerData.slot,
				"prevRandSeed", headerData.prevRandSeed,
				"consensusOrder", broadcastData.order,
			)
		}
	}
	d.mutDataForBroadcast.RUnlock()

	for _, a := range alarmsToAdd {
		d.alarm.Add(d.alarmExpired, a.duration, a.id)
	}
}

func (d *delayedBlockBroadcaster) alarmExpired(alarmID string) {
	// extract header from alarm id
	headerHash, err := hex.DecodeString(strings.TrimPrefix(alarmID, prefixDelayDataAlarm))
	if err != nil {
		log.Error("delayedBlockBroadcaster.alarmExpired", "error", err.Error(),
			"headerHash", headerHash,
			"alarmID", alarmID,
		)
		return
	}

	// extract data to broadcast from list and remove from it
	d.mutDataForBroadcast.Lock()
	dataToBroadcast := make([]*delayedBroadcastData, 0)
	for i, broadcastData := range d.valBroadcastData {
		if bytes.Equal(broadcastData.headerHash, headerHash) {
			log.Debug("delayedBlockBroadcaster.alarmExpired: validator broadcasts block data (with delay) instead of leader",
				"headerHash", headerHash,
				"alarmID", alarmID,
			)
			dataToBroadcast = append(dataToBroadcast, broadcastData)
			d.valBroadcastData = append(d.valBroadcastData[:i], d.valBroadcastData[i+1:]...)
			break
		}
	}
	d.mutDataForBroadcast.Unlock()

	// perform broadcast
	if len(dataToBroadcast) > 0 {
		d.broadcastDelayedData(dataToBroadcast)
	}
}

func (d *delayedBlockBroadcaster) broadcastDelayedData(broadcastData []*delayedBroadcastData) {
	for _, bData := range broadcastData {
		go func(transactions [][]byte) {
			d.broadcastBlockData(transactions, common.ExtraDelayForBroadcastBlockInfo)
		}(bData.transactions)
	}
}

func (d *delayedBlockBroadcaster) broadcastBlockData(
	transactions [][]byte,
	delay time.Duration,
) {
	time.Sleep(delay)

	err := d.broadcastTxsData(transactions)
	if err != nil {
		log.Error("broadcastBlockData.broadcastTxsData", "error", err.Error())
	}
}

func (d *delayedBlockBroadcaster) SetBroadcastHandlers(
	txsBroadcast func(txData [][]byte) error,
	headerBroadcast func(header data.HeaderHandler) error,
) error {
	if txsBroadcast == nil {
		return common.ErrNilParameter
	}

	if headerBroadcast == nil {
		return common.ErrNilParameter
	}

	d.mutDataForBroadcast.Lock()
	defer d.mutDataForBroadcast.Unlock()

	d.broadcastHeader = headerBroadcast
	d.broadcastTxsData = txsBroadcast

	return nil
}

func getDataFromBlock(headerHandler data.HeaderHandler) (*headerDataForValidator, error) {
	header, ok := headerHandler.(*block.Block)
	if !ok {
		return nil, common.ErrNilHeader
	}

	headerData := &headerDataForValidator{
		slot:         header.GetSlot(),
		prevRandSeed: header.GetHeader().GetPrevRandSeed(),
	}

	return headerData, nil
}

func (d *delayedBlockBroadcaster) headerReceived(headerHandler data.HeaderHandler, headerHash []byte) {
	d.mutDataForBroadcast.RLock()
	defer d.mutDataForBroadcast.RUnlock()

	if len(d.delayedBroadcastData) == 0 && len(d.valBroadcastData) == 0 {
		return
	}

	dataForValidators, err := getDataFromBlock(headerHandler)
	if err != nil {
		log.Error("delayedBlockBroadcaster.headerReceived", "error", err.Error(),
			"headerHash", headerHash,
		)
		return
	}
	if len(headerHash) == 0 {
		log.Trace("delayedBlockBroadcaster.headerReceived: header received with no shardData for current shard",
			"headerHash", headerHash,
		)
		return
	}

	log.Trace("delayedBlockBroadcaster.headerReceived", "headerHash", headerHash)

	go d.scheduleValidatorBroadcast(dataForValidators)
	go d.broadcastDataForHeaders(headerHash)
}

func (d *delayedBlockBroadcaster) broadcastDataForHeaders(headerHash []byte) {
	d.mutDataForBroadcast.RLock()
	if len(d.delayedBroadcastData) == 0 {
		d.mutDataForBroadcast.RUnlock()
		return
	}
	d.mutDataForBroadcast.RUnlock()

	time.Sleep(common.ExtraDelayForBroadcastBlockInfo)

	d.mutDataForBroadcast.Lock()
	dataToBroadcast := make([]*delayedBroadcastData, 0)

	// retrieve data to broadcast using header hash, and remove it from the data list
	for i := len(d.delayedBroadcastData) - 1; i >= 0; i-- {
		if bytes.Equal(d.delayedBroadcastData[i].headerHash, headerHash) {
			log.Debug("delayedBlockBroadcaster.broadcastDataForHeaders: leader broadcasts block data",
				"headerHash", headerHash,
			)
			dataToBroadcast = append(dataToBroadcast, d.delayedBroadcastData[:i+1]...)
			d.delayedBroadcastData = d.delayedBroadcastData[i+1:]
			break
		}
	}
	d.mutDataForBroadcast.Unlock()

	// broadcast
	if len(dataToBroadcast) > 0 {
		d.broadcastDelayedData(dataToBroadcast)
	}
}

func (d *delayedBlockBroadcaster) interceptedHeader(_ string, headerHash []byte, header interface{}) {
	headerHandler, ok := header.(data.HeaderHandler)
	if !ok {
		log.Warn("delayedBlockBroadcaster.interceptedHeader", "error", process.ErrWrongTypeAssertion,
			"headerHash", headerHash,
		)
		return
	}

	d.mutHeadersCache.Lock()
	d.cacheHeaders.Put(headerHash, struct{}{}, 0)
	d.mutHeadersCache.Unlock()

	log.Trace("delayedBlockBroadcaster.interceptedHeader",
		"headerHash", headerHash,
		"slot", headerHandler.GetSlot(),
		"prevRandSeed", headerHandler.GetPrevRandSeed())

	alarmsToCancel := make([]string, 0)
	d.mutDataForBroadcast.Lock()
	for i, broadcastData := range d.valHeaderBroadcastData {
		samePrevRandSeed := bytes.Equal(broadcastData.header.GetPrevRandSeed(), headerHandler.GetPrevRandSeed())
		sameSlot := broadcastData.header.GetSlot() == headerHandler.GetSlot()
		sameHeader := samePrevRandSeed && sameSlot

		if sameHeader {
			d.valHeaderBroadcastData = append(d.valHeaderBroadcastData[:i], d.valHeaderBroadcastData[i+1:]...)
			// leader has already broadcast the header, so we can cancel the header alarm
			alarmID := prefixHeaderAlarm + hex.EncodeToString(headerHash)
			alarmsToCancel = append(alarmsToCancel, alarmID)
			log.Trace("delayedBlockBroadcaster.interceptedHeader: leader has broadcast header, validator cancelling alarm",
				"headerHash", headerHash,
				"alarmID", alarmID,
			)
			break
		}
	}
	d.mutDataForBroadcast.Unlock()

	for _, alarmID := range alarmsToCancel {
		d.alarm.Cancel(alarmID)
	}
}

func (d *delayedBlockBroadcaster) registerHeaderInterceptorCallback(
	cb func(topic string, hash []byte, data interface{}),
) error {
	interceptor, err := d.interceptorsContainer.Get(common.BlocksTopic)
	if err != nil {
		return err
	}

	d.interceptorsContainer.Iterate(func(topic string, interceptor process.Interceptor) bool {
		log.Debug("delayedBlockBroadcaster.registerHeaderInterceptorCallback", "topic", topic)
		// should continue
		return true
	})

	interceptor.RegisterHandler(cb)

	return nil
}

func (d *delayedBlockBroadcaster) ScheduleValidatorBroadcast(dataForValidator *HeaderDataForValidator) {
	d.scheduleValidatorBroadcast(&headerDataForValidator{
		slot:         dataForValidator.Slot,
		prevRandSeed: dataForValidator.PrevRandSeed,
	})
}

func (d *delayedBlockBroadcaster) HeaderReceived(headerHandler data.HeaderHandler, hash []byte) {
	d.headerReceived(headerHandler, hash)
}

// AlarmExpired -
func (d *delayedBlockBroadcaster) AlarmExpired(headerHash string) {
	d.alarmExpired(headerHash)
}

// InterceptedHeaderData -
func (d *delayedBlockBroadcaster) InterceptedHeaderData(hash []byte, header interface{}) {
	d.interceptedHeader("", hash, header)
}

func (d *delayedBlockBroadcaster) GetValidatorBroadcastData() []*delayedBroadcastData {
	d.mutDataForBroadcast.RLock()
	copyValBroadcastData := make([]*delayedBroadcastData, len(d.valBroadcastData))
	copy(copyValBroadcastData, d.valBroadcastData)
	d.mutDataForBroadcast.RUnlock()

	return copyValBroadcastData
}

func (d *delayedBlockBroadcaster) GetValidatorHeaderBroadcastData() []*validatorHeaderBroadcastData {
	d.mutDataForBroadcast.RLock()
	copyValHeaderBroadcastData := make([]*validatorHeaderBroadcastData, len(d.valHeaderBroadcastData))
	copy(copyValHeaderBroadcastData, d.valHeaderBroadcastData)
	d.mutDataForBroadcast.RUnlock()

	return copyValHeaderBroadcastData
}

func (d *delayedBlockBroadcaster) GetLeaderBroadcastData() []*delayedBroadcastData {
	d.mutDataForBroadcast.RLock()
	copyDelayBroadcastData := make([]*delayedBroadcastData, len(d.delayedBroadcastData))
	copy(copyDelayBroadcastData, d.delayedBroadcastData)
	d.mutDataForBroadcast.RUnlock()

	return copyDelayBroadcastData
}

func CreateDelayBroadcastDataForLeader(
	headerHash []byte,
	header data.HeaderHandler,
	transactionsData [][]byte,
) *delayedBroadcastData {
	return &delayedBroadcastData{
		headerHash:   headerHash,
		header:       header,
		transactions: transactionsData,
		order:        1,
	}
}

// CreateDelayBroadcastDataForValidator creates the delayed broadcast data
func CreateDelayBroadcastDataForValidator(
	headerHash []byte,
	header data.HeaderHandler,
	transactionsData [][]byte,
	order uint32,
) *delayedBroadcastData {
	return &delayedBroadcastData{
		headerHash:   headerHash,
		header:       header,
		transactions: transactionsData,
		order:        order,
	}
}

func CreateValidatorHeaderBroadcastData(
	headerHash []byte,
	header data.HeaderHandler,
	transactionsData [][]byte,
	order uint32,
) *validatorHeaderBroadcastData {
	return &validatorHeaderBroadcastData{
		headerHash:       headerHash,
		header:           header,
		transactionsData: transactionsData,
		order:            order,
	}
}

// Close closes all the started infinite looping goroutines and subcomponents
func (d *delayedBlockBroadcaster) Close() {
	d.alarm.Close()
}
