package mock

import (
	"crypto/elliptic"
	"io"
	"math/big"

	"github.com/klever-io/klever-go/kvm/vmhost"
	"github.com/klever-io/klever-go/vmcommon"
)

var _ vmhost.ManagedTypesContext = (*ManagedTypesContextMock)(nil)

// ManagedTypesContextMock -
type ManagedTypesContextMock struct {
	InitStateCalled                                func()
	PushStateCalled                                func()
	PopSetActiveStateCalled                        func()
	PopDiscardCalled                               func()
	ClearStateStackCalled                          func()
	GetRandReaderCalled                            func() io.Reader
	ConsumeGasForThisBigIntNumberOfBytesCalled     func(byteLen *big.Int)
	ConsumeGasForThisIntNumberOfBytesCalled        func(byteLen int)
	ConsumeGasForBytesCalled                       func(bytes []byte)
	ConsumeGasForBigIntCopyCalled                  func(values ...*big.Int)
	ConsumeGasForBigFloatCopyCalled                func(values ...*big.Float)
	NewBigIntCalled                                func(value *big.Int) int32
	NewBigIntFromInt64Called                       func(int64Value int64) int32
	GetBigIntOrCreateCalled                        func(handle int32) *big.Int
	GetBigIntCalled                                func(id int32) (*big.Int, error)
	GetTwoBigIntCalled                             func(handle1 int32, handle2 int32) (*big.Int, *big.Int, error)
	PutBigFloatCalled                              func(value *big.Float) (int32, error)
	BigFloatPrecIsNotValidCalled                   func(precision uint) bool
	BigFloatExpIsNotValidCalled                    func(exponent int) bool
	EncodedBigFloatIsNotValidCalled                func(encodedBigFloat []byte) bool
	GetBigFloatOrCreateCalled                      func(handle int32) (*big.Float, error)
	GetBigFloatCalled                              func(handle int32) (*big.Float, error)
	GetTwoBigFloatsCalled                          func(handle1 int32, handle2 int32) (*big.Float, *big.Float, error)
	PutEllipticCurveCalled                         func(ec *elliptic.CurveParams) int32
	GetEllipticCurveCalled                         func(handle int32) (*elliptic.CurveParams, error)
	GetEllipticCurveSizeOfFieldCalled              func(ecHandle int32) int32
	Get100xCurveGasCostMultiplierCalled            func(ecHandle int32) int32
	GetScalarMult100xCurveGasCostMultiplierCalled  func(ecHandle int32) int32
	GetUCompressed100xCurveGasCostMultiplierCalled func(ecHandle int32) int32
	GetPrivateKeyByteLengthECCalled                func(ecHandle int32) int32
	NewManagedBufferCalled                         func() int32
	NewManagedBufferFromBytesCalled                func(bytes []byte) int32
	SetBytesCalled                                 func(mBufferHandle int32, bytes []byte)
	GetBytesCalled                                 func(mBufferHandle int32) ([]byte, error)
	AppendBytesCalled                              func(mBufferHandle int32, bytes []byte) bool
	GetLengthCalled                                func(mBufferHandle int32) int32
	GetSliceCalled                                 func(mBufferHandle int32, startPosition int32, lengthOfSlice int32) ([]byte, error)
	DeleteSliceCalled                              func(mBufferHandle int32, startPosition int32, lengthOfSlice int32) ([]byte, error)
	InsertSliceCalled                              func(mBufferHandle int32, startPosition int32, slice []byte) ([]byte, error)
	ReadManagedVecOfManagedBuffersCalled           func(managedVecHandle int32) ([][]byte, uint64, error)
	WriteManagedVecOfManagedBuffersCalled          func(data [][]byte, destinationHandle int32)
	NewManagedMapCalled                            func() int32
	ManagedMapPutCalled                            func(mMapHandle int32, keyHandle int32, valueHandle int32) error
	ManagedMapGetCalled                            func(mMapHandle int32, keyHandle int32, outValueHandle int32) error
	ManagedMapRemoveCalled                         func(mMapHandle int32, keyHandle int32, outValueHandle int32) error
	ManagedMapContainsCalled                       func(mMapHandle int32, keyHandle int32) (bool, error)
	GetBackTransfersCalled                         func() ([]*vmcommon.KDATransfer, *big.Int)
	AddValueOnlyBackTransferCalled                 func(value *big.Int)
	AddBackTransfersCalled                         func(transfers []*vmcommon.KDATransfer)
}

func (m *ManagedTypesContextMock) InitState() {
	if m.InitStateCalled != nil {
		m.InitStateCalled()
	}
}

func (m *ManagedTypesContextMock) PushState() {
	if m.PushStateCalled != nil {
		m.PushStateCalled()
	}
}

func (m *ManagedTypesContextMock) PopSetActiveState() {
	if m.PopSetActiveStateCalled != nil {
		m.PopSetActiveStateCalled()
	}
}

func (m *ManagedTypesContextMock) PopDiscard() {
	if m.PopDiscardCalled != nil {
		m.PopDiscardCalled()
	}
}

func (m *ManagedTypesContextMock) ClearStateStack() {
	if m.ClearStateStackCalled != nil {
		m.ClearStateStackCalled()
	}
}

func (m *ManagedTypesContextMock) GetRandReader() io.Reader {
	if m.GetRandReaderCalled != nil {
		return m.GetRandReaderCalled()
	}
	return nil
}

func (m *ManagedTypesContextMock) ConsumeGasForThisBigIntNumberOfBytes(byteLen *big.Int) {
	if m.ConsumeGasForThisBigIntNumberOfBytesCalled != nil {
		m.ConsumeGasForThisBigIntNumberOfBytesCalled(byteLen)
	}
}

func (m *ManagedTypesContextMock) ConsumeGasForThisIntNumberOfBytes(byteLen int) {
	if m.ConsumeGasForThisIntNumberOfBytesCalled != nil {
		m.ConsumeGasForThisIntNumberOfBytesCalled(byteLen)
	}
}

func (m *ManagedTypesContextMock) ConsumeGasForBytes(bytes []byte) {
	if m.ConsumeGasForBytesCalled != nil {
		m.ConsumeGasForBytesCalled(bytes)
	}
}

func (m *ManagedTypesContextMock) ConsumeGasForBigIntCopy(values ...*big.Int) {
	if m.ConsumeGasForBigIntCopyCalled != nil {
		m.ConsumeGasForBigIntCopyCalled(values...)
	}
}

func (m *ManagedTypesContextMock) ConsumeGasForBigFloatCopy(values ...*big.Float) {
	if m.ConsumeGasForBigFloatCopyCalled != nil {
		m.ConsumeGasForBigFloatCopyCalled(values...)
	}
}

func (m *ManagedTypesContextMock) NewBigInt(value *big.Int) int32 {
	if m.NewBigIntCalled != nil {
		return m.NewBigIntCalled(value)
	}
	return 0
}

func (m *ManagedTypesContextMock) NewBigIntFromInt64(int64Value int64) int32 {
	if m.NewBigIntFromInt64Called != nil {
		return m.NewBigIntFromInt64Called(int64Value)
	}
	return 0
}

func (m *ManagedTypesContextMock) GetBigIntOrCreate(handle int32) *big.Int {
	if m.GetBigIntOrCreateCalled != nil {
		return m.GetBigIntOrCreateCalled(handle)
	}
	return big.NewInt(0)
}

func (m *ManagedTypesContextMock) GetBigInt(id int32) (*big.Int, error) {
	if m.GetBigIntCalled != nil {
		return m.GetBigIntCalled(id)
	}
	return big.NewInt(0), nil
}

func (m *ManagedTypesContextMock) GetTwoBigInt(handle1 int32, handle2 int32) (*big.Int, *big.Int, error) {
	if m.GetTwoBigIntCalled != nil {
		return m.GetTwoBigIntCalled(handle1, handle2)
	}
	return big.NewInt(0), big.NewInt(0), nil
}

func (m *ManagedTypesContextMock) PutBigFloat(value *big.Float) (int32, error) {
	if m.PutBigFloatCalled != nil {
		return m.PutBigFloatCalled(value)
	}
	return 0, nil
}

func (m *ManagedTypesContextMock) BigFloatPrecIsNotValid(precision uint) bool {
	if m.BigFloatPrecIsNotValidCalled != nil {
		return m.BigFloatPrecIsNotValidCalled(precision)
	}
	return false
}

func (m *ManagedTypesContextMock) BigFloatExpIsNotValid(exponent int) bool {
	if m.BigFloatExpIsNotValidCalled != nil {
		return m.BigFloatExpIsNotValidCalled(exponent)
	}
	return false
}

func (m *ManagedTypesContextMock) EncodedBigFloatIsNotValid(encodedBigFloat []byte) bool {
	if m.EncodedBigFloatIsNotValidCalled != nil {
		return m.EncodedBigFloatIsNotValidCalled(encodedBigFloat)
	}
	return false
}

func (m *ManagedTypesContextMock) GetBigFloatOrCreate(handle int32) (*big.Float, error) {
	if m.GetBigFloatOrCreateCalled != nil {
		return m.GetBigFloatOrCreateCalled(handle)
	}
	return big.NewFloat(0), nil
}

func (m *ManagedTypesContextMock) GetBigFloat(handle int32) (*big.Float, error) {
	if m.GetBigFloatCalled != nil {
		return m.GetBigFloatCalled(handle)
	}
	return big.NewFloat(0), nil
}

func (m *ManagedTypesContextMock) GetTwoBigFloats(handle1 int32, handle2 int32) (*big.Float, *big.Float, error) {
	if m.GetTwoBigFloatsCalled != nil {
		return m.GetTwoBigFloatsCalled(handle1, handle2)
	}
	return big.NewFloat(0), big.NewFloat(0), nil
}

func (m *ManagedTypesContextMock) PutEllipticCurve(ec *elliptic.CurveParams) int32 {
	if m.PutEllipticCurveCalled != nil {
		return m.PutEllipticCurveCalled(ec)
	}
	return 0
}

func (m *ManagedTypesContextMock) GetEllipticCurve(handle int32) (*elliptic.CurveParams, error) {
	if m.GetEllipticCurveCalled != nil {
		return m.GetEllipticCurveCalled(handle)
	}
	return nil, nil
}

func (m *ManagedTypesContextMock) GetEllipticCurveSizeOfField(ecHandle int32) int32 {
	if m.GetEllipticCurveSizeOfFieldCalled != nil {
		return m.GetEllipticCurveSizeOfFieldCalled(ecHandle)
	}
	return 0
}

func (m *ManagedTypesContextMock) Get100xCurveGasCostMultiplier(ecHandle int32) int32 {
	if m.Get100xCurveGasCostMultiplierCalled != nil {
		return m.Get100xCurveGasCostMultiplierCalled(ecHandle)
	}
	return 0
}

func (m *ManagedTypesContextMock) GetScalarMult100xCurveGasCostMultiplier(ecHandle int32) int32 {
	if m.GetScalarMult100xCurveGasCostMultiplierCalled != nil {
		return m.GetScalarMult100xCurveGasCostMultiplierCalled(ecHandle)
	}
	return 0
}

func (m *ManagedTypesContextMock) GetUCompressed100xCurveGasCostMultiplier(ecHandle int32) int32 {
	if m.GetUCompressed100xCurveGasCostMultiplierCalled != nil {
		return m.GetUCompressed100xCurveGasCostMultiplierCalled(ecHandle)
	}
	return 0
}

func (m *ManagedTypesContextMock) GetPrivateKeyByteLengthEC(ecHandle int32) int32 {
	if m.GetPrivateKeyByteLengthECCalled != nil {
		return m.GetPrivateKeyByteLengthECCalled(ecHandle)
	}
	return 0
}

func (m *ManagedTypesContextMock) NewManagedBuffer() int32 {
	if m.NewManagedBufferCalled != nil {
		return m.NewManagedBufferCalled()
	}
	return 0
}

func (m *ManagedTypesContextMock) NewManagedBufferFromBytes(bytes []byte) int32 {
	if m.NewManagedBufferFromBytesCalled != nil {
		return m.NewManagedBufferFromBytesCalled(bytes)
	}
	return 0
}

func (m *ManagedTypesContextMock) SetBytes(mBufferHandle int32, bytes []byte) {
	if m.SetBytesCalled != nil {
		m.SetBytesCalled(mBufferHandle, bytes)
	}
}

func (m *ManagedTypesContextMock) GetBytes(mBufferHandle int32) ([]byte, error) {
	if m.GetBytesCalled != nil {
		return m.GetBytesCalled(mBufferHandle)
	}
	return nil, nil
}

func (m *ManagedTypesContextMock) AppendBytes(mBufferHandle int32, bytes []byte) bool {
	if m.AppendBytesCalled != nil {
		return m.AppendBytesCalled(mBufferHandle, bytes)
	}
	return false
}

func (m *ManagedTypesContextMock) GetLength(mBufferHandle int32) int32 {
	if m.GetLengthCalled != nil {
		return m.GetLengthCalled(mBufferHandle)
	}
	return 0
}

func (m *ManagedTypesContextMock) GetSlice(mBufferHandle int32, startPosition int32, lengthOfSlice int32) ([]byte, error) {
	if m.GetSliceCalled != nil {
		return m.GetSliceCalled(mBufferHandle, startPosition, lengthOfSlice)
	}
	return nil, nil
}

func (m *ManagedTypesContextMock) DeleteSlice(mBufferHandle int32, startPosition int32, lengthOfSlice int32) ([]byte, error) {
	if m.DeleteSliceCalled != nil {
		return m.DeleteSliceCalled(mBufferHandle, startPosition, lengthOfSlice)
	}
	return nil, nil
}

func (m *ManagedTypesContextMock) InsertSlice(mBufferHandle int32, startPosition int32, slice []byte) ([]byte, error) {
	if m.InsertSliceCalled != nil {
		return m.InsertSliceCalled(mBufferHandle, startPosition, slice)
	}
	return nil, nil
}

func (m *ManagedTypesContextMock) ReadManagedVecOfManagedBuffers(managedVecHandle int32) ([][]byte, uint64, error) {
	if m.ReadManagedVecOfManagedBuffersCalled != nil {
		return m.ReadManagedVecOfManagedBuffersCalled(managedVecHandle)
	}
	return nil, 0, nil
}

func (m *ManagedTypesContextMock) WriteManagedVecOfManagedBuffers(data [][]byte, destinationHandle int32) {
	if m.WriteManagedVecOfManagedBuffersCalled != nil {
		m.WriteManagedVecOfManagedBuffersCalled(data, destinationHandle)
	}
}

func (m *ManagedTypesContextMock) NewManagedMap() int32 {
	if m.NewManagedMapCalled != nil {
		return m.NewManagedMapCalled()
	}
	return 0
}

func (m *ManagedTypesContextMock) ManagedMapPut(mMapHandle int32, keyHandle int32, valueHandle int32) error {
	if m.ManagedMapPutCalled != nil {
		return m.ManagedMapPutCalled(mMapHandle, keyHandle, valueHandle)
	}
	return nil
}

func (m *ManagedTypesContextMock) ManagedMapGet(mMapHandle int32, keyHandle int32, outValueHandle int32) error {
	if m.ManagedMapGetCalled != nil {
		return m.ManagedMapGetCalled(mMapHandle, keyHandle, outValueHandle)
	}
	return nil
}

func (m *ManagedTypesContextMock) ManagedMapRemove(mMapHandle int32, keyHandle int32, outValueHandle int32) error {
	if m.ManagedMapRemoveCalled != nil {
		return m.ManagedMapRemoveCalled(mMapHandle, keyHandle, outValueHandle)
	}
	return nil
}

func (m *ManagedTypesContextMock) ManagedMapContains(mMapHandle int32, keyHandle int32) (bool, error) {
	if m.ManagedMapContainsCalled != nil {
		return m.ManagedMapContainsCalled(mMapHandle, keyHandle)
	}
	return false, nil
}

func (m *ManagedTypesContextMock) GetBackTransfers() ([]*vmcommon.KDATransfer, *big.Int) {
	if m.GetBackTransfersCalled != nil {
		return m.GetBackTransfersCalled()
	}
	return nil, big.NewInt(0)
}

func (m *ManagedTypesContextMock) AddValueOnlyBackTransfer(value *big.Int) {
	if m.AddValueOnlyBackTransferCalled != nil {
		m.AddValueOnlyBackTransferCalled(value)
	}
}

func (m *ManagedTypesContextMock) AddBackTransfers(transfers []*vmcommon.KDATransfer) {
	if m.AddBackTransfersCalled != nil {
		m.AddBackTransfersCalled(transfers)
	}
}

// IsInterfaceNil returns true if there is no value under the interface
func (m *ManagedTypesContextMock) IsInterfaceNil() bool {
	return m == nil
}
