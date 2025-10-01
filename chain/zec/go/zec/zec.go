package zec

// #include <zec.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"unsafe"
)

type RustBuffer = C.RustBuffer

type RustBufferI interface {
	AsReader() *bytes.Reader
	Free()
	ToGoBytes() []byte
	Data() unsafe.Pointer
	Len() int
	Capacity() int
}

func RustBufferFromExternal(b RustBufferI) RustBuffer {
	return RustBuffer{
		capacity: C.int(b.Capacity()),
		len:      C.int(b.Len()),
		data:     (*C.uchar)(b.Data()),
	}
}

func (cb RustBuffer) Capacity() int {
	return int(cb.capacity)
}

func (cb RustBuffer) Len() int {
	return int(cb.len)
}

func (cb RustBuffer) Data() unsafe.Pointer {
	return unsafe.Pointer(cb.data)
}

func (cb RustBuffer) AsReader() *bytes.Reader {
	b := unsafe.Slice((*byte)(cb.data), C.int(cb.len))
	return bytes.NewReader(b)
}

func (cb RustBuffer) Free() {
	rustCall(func(status *C.RustCallStatus) bool {
		C.ffi_zec_rustbuffer_free(cb, status)
		return false
	})
}

func (cb RustBuffer) ToGoBytes() []byte {
	return C.GoBytes(unsafe.Pointer(cb.data), C.int(cb.len))
}

func stringToRustBuffer(str string) RustBuffer {
	return bytesToRustBuffer([]byte(str))
}

func bytesToRustBuffer(b []byte) RustBuffer {
	if len(b) == 0 {
		return RustBuffer{}
	}
	// We can pass the pointer along here, as it is pinned
	// for the duration of this call
	foreign := C.ForeignBytes{
		len:  C.int(len(b)),
		data: (*C.uchar)(unsafe.Pointer(&b[0])),
	}

	return rustCall(func(status *C.RustCallStatus) RustBuffer {
		return C.ffi_zec_rustbuffer_from_bytes(foreign, status)
	})
}

type BufLifter[GoType any] interface {
	Lift(value RustBufferI) GoType
}

type BufLowerer[GoType any] interface {
	Lower(value GoType) RustBuffer
}

type FfiConverter[GoType any, FfiType any] interface {
	Lift(value FfiType) GoType
	Lower(value GoType) FfiType
}

type BufReader[GoType any] interface {
	Read(reader io.Reader) GoType
}

type BufWriter[GoType any] interface {
	Write(writer io.Writer, value GoType)
}

type FfiRustBufConverter[GoType any, FfiType any] interface {
	FfiConverter[GoType, FfiType]
	BufReader[GoType]
}

func LowerIntoRustBuffer[GoType any](bufWriter BufWriter[GoType], value GoType) RustBuffer {
	// This might be not the most efficient way but it does not require knowing allocation size
	// beforehand
	var buffer bytes.Buffer
	bufWriter.Write(&buffer, value)

	bytes, err := io.ReadAll(&buffer)
	if err != nil {
		panic(fmt.Errorf("reading written data: %w", err))
	}
	return bytesToRustBuffer(bytes)
}

func LiftFromRustBuffer[GoType any](bufReader BufReader[GoType], rbuf RustBufferI) GoType {
	defer rbuf.Free()
	reader := rbuf.AsReader()
	item := bufReader.Read(reader)
	if reader.Len() > 0 {
		// TODO: Remove this
		leftover, _ := io.ReadAll(reader)
		panic(fmt.Errorf("Junk remaining in buffer after lifting: %s", string(leftover)))
	}
	return item
}

func rustCallWithError[U any](converter BufLifter[error], callback func(*C.RustCallStatus) U) (U, error) {
	var status C.RustCallStatus
	returnValue := callback(&status)
	err := checkCallStatus(converter, status)

	return returnValue, err
}

func checkCallStatus(converter BufLifter[error], status C.RustCallStatus) error {
	switch status.code {
	case 0:
		return nil
	case 1:
		return converter.Lift(status.errorBuf)
	case 2:
		// when the rust code sees a panic, it tries to construct a rustbuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(status.errorBuf)))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		return fmt.Errorf("unknown status code: %d", status.code)
	}
}

func checkCallStatusUnknown(status C.RustCallStatus) error {
	switch status.code {
	case 0:
		return nil
	case 1:
		panic(fmt.Errorf("function not returning an error returned an error"))
	case 2:
		// when the rust code sees a panic, it tries to construct a rustbuffer
		// with the message.  but if that code panics, then it just sends back
		// an empty buffer.
		if status.errorBuf.len > 0 {
			panic(fmt.Errorf("%s", FfiConverterStringINSTANCE.Lift(status.errorBuf)))
		} else {
			panic(fmt.Errorf("Rust panicked while handling Rust panic"))
		}
	default:
		return fmt.Errorf("unknown status code: %d", status.code)
	}
}

func rustCall[U any](callback func(*C.RustCallStatus) U) U {
	returnValue, err := rustCallWithError(nil, callback)
	if err != nil {
		panic(err)
	}
	return returnValue
}

func writeInt8(writer io.Writer, value int8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint8(writer io.Writer, value uint8) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt16(writer io.Writer, value int16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint16(writer io.Writer, value uint16) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt32(writer io.Writer, value int32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint32(writer io.Writer, value uint32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeInt64(writer io.Writer, value int64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeUint64(writer io.Writer, value uint64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat32(writer io.Writer, value float32) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func writeFloat64(writer io.Writer, value float64) {
	if err := binary.Write(writer, binary.BigEndian, value); err != nil {
		panic(err)
	}
}

func readInt8(reader io.Reader) int8 {
	var result int8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint8(reader io.Reader) uint8 {
	var result uint8
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt16(reader io.Reader) int16 {
	var result int16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint16(reader io.Reader) uint16 {
	var result uint16
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt32(reader io.Reader) int32 {
	var result int32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint32(reader io.Reader) uint32 {
	var result uint32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readInt64(reader io.Reader) int64 {
	var result int64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readUint64(reader io.Reader) uint64 {
	var result uint64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat32(reader io.Reader) float32 {
	var result float32
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func readFloat64(reader io.Reader) float64 {
	var result float64
	if err := binary.Read(reader, binary.BigEndian, &result); err != nil {
		panic(err)
	}
	return result
}

func init() {

	uniffiCheckChecksums()
}

func uniffiCheckChecksums() {
	// Get the bindings contract version from our ComponentInterface
	bindingsContractVersion := 24
	// Get the scaffolding contract version by calling the into the dylib
	scaffoldingContractVersion := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint32_t {
		return C.ffi_zec_uniffi_contract_version(uniffiStatus)
	})
	if bindingsContractVersion != int(scaffoldingContractVersion) {
		// If this happens try cleaning and rebuilding your project
		panic("zec: UniFFI contract version mismatch")
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zec_checksum_func_apply_signatures(uniffiStatus)
		})
		if checksum != 60547 {
			// If this happens try cleaning and rebuilding your project
			panic("zec: uniffi_zec_checksum_func_apply_signatures: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zec_checksum_func_build_ptx(uniffiStatus)
		})
		if checksum != 317 {
			// If this happens try cleaning and rebuilding your project
			panic("zec: uniffi_zec_checksum_func_build_ptx: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zec_checksum_func_compute_txid(uniffiStatus)
		})
		if checksum != 4560 {
			// If this happens try cleaning and rebuilding your project
			panic("zec: uniffi_zec_checksum_func_compute_txid: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zec_checksum_func_get_ovk(uniffiStatus)
		})
		if checksum != 61727 {
			// If this happens try cleaning and rebuilding your project
			panic("zec: uniffi_zec_checksum_func_get_ovk: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zec_checksum_func_init_zec(uniffiStatus)
		})
		if checksum != 23758 {
			// If this happens try cleaning and rebuilding your project
			panic("zec: uniffi_zec_checksum_func_init_zec: UniFFI API checksum mismatch")
		}
	}
	{
		checksum := rustCall(func(uniffiStatus *C.RustCallStatus) C.uint16_t {
			return C.uniffi_zec_checksum_func_validate_address(uniffiStatus)
		})
		if checksum != 57951 {
			// If this happens try cleaning and rebuilding your project
			panic("zec: uniffi_zec_checksum_func_validate_address: UniFFI API checksum mismatch")
		}
	}
}

type FfiConverterUint32 struct{}

var FfiConverterUint32INSTANCE = FfiConverterUint32{}

func (FfiConverterUint32) Lower(value uint32) C.uint32_t {
	return C.uint32_t(value)
}

func (FfiConverterUint32) Write(writer io.Writer, value uint32) {
	writeUint32(writer, value)
}

func (FfiConverterUint32) Lift(value C.uint32_t) uint32 {
	return uint32(value)
}

func (FfiConverterUint32) Read(reader io.Reader) uint32 {
	return readUint32(reader)
}

type FfiDestroyerUint32 struct{}

func (FfiDestroyerUint32) Destroy(_ uint32) {}

type FfiConverterUint64 struct{}

var FfiConverterUint64INSTANCE = FfiConverterUint64{}

func (FfiConverterUint64) Lower(value uint64) C.uint64_t {
	return C.uint64_t(value)
}

func (FfiConverterUint64) Write(writer io.Writer, value uint64) {
	writeUint64(writer, value)
}

func (FfiConverterUint64) Lift(value C.uint64_t) uint64 {
	return uint64(value)
}

func (FfiConverterUint64) Read(reader io.Reader) uint64 {
	return readUint64(reader)
}

type FfiDestroyerUint64 struct{}

func (FfiDestroyerUint64) Destroy(_ uint64) {}

type FfiConverterString struct{}

var FfiConverterStringINSTANCE = FfiConverterString{}

func (FfiConverterString) Lift(rb RustBufferI) string {
	defer rb.Free()
	reader := rb.AsReader()
	b, err := io.ReadAll(reader)
	if err != nil {
		panic(fmt.Errorf("reading reader: %w", err))
	}
	return string(b)
}

func (FfiConverterString) Read(reader io.Reader) string {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading string, expected %d, read %d", length, read_length))
	}
	return string(buffer)
}

func (FfiConverterString) Lower(value string) RustBuffer {
	return stringToRustBuffer(value)
}

func (FfiConverterString) Write(writer io.Writer, value string) {
	if len(value) > math.MaxInt32 {
		panic("String is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := io.WriteString(writer, value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing string, expected %d, written %d", len(value), write_length))
	}
}

type FfiDestroyerString struct{}

func (FfiDestroyerString) Destroy(_ string) {}

type FfiConverterBytes struct{}

var FfiConverterBytesINSTANCE = FfiConverterBytes{}

func (c FfiConverterBytes) Lower(value []byte) RustBuffer {
	return LowerIntoRustBuffer[[]byte](c, value)
}

func (c FfiConverterBytes) Write(writer io.Writer, value []byte) {
	if len(value) > math.MaxInt32 {
		panic("[]byte is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	write_length, err := writer.Write(value)
	if err != nil {
		panic(err)
	}
	if write_length != len(value) {
		panic(fmt.Errorf("bad write length when writing []byte, expected %d, written %d", len(value), write_length))
	}
}

func (c FfiConverterBytes) Lift(rb RustBufferI) []byte {
	return LiftFromRustBuffer[[]byte](c, rb)
}

func (c FfiConverterBytes) Read(reader io.Reader) []byte {
	length := readInt32(reader)
	buffer := make([]byte, length)
	read_length, err := reader.Read(buffer)
	if err != nil {
		panic(err)
	}
	if read_length != int(length) {
		panic(fmt.Errorf("bad read length when reading []byte, expected %d, read %d", length, read_length))
	}
	return buffer
}

type FfiDestroyerBytes struct{}

func (FfiDestroyerBytes) Destroy(_ []byte) {}

type Output struct {
	Address string
	Amount  uint64
	Memo    string
}

func (r *Output) Destroy() {
	FfiDestroyerString{}.Destroy(r.Address)
	FfiDestroyerUint64{}.Destroy(r.Amount)
	FfiDestroyerString{}.Destroy(r.Memo)
}

type FfiConverterTypeOutput struct{}

var FfiConverterTypeOutputINSTANCE = FfiConverterTypeOutput{}

func (c FfiConverterTypeOutput) Lift(rb RustBufferI) Output {
	return LiftFromRustBuffer[Output](c, rb)
}

func (c FfiConverterTypeOutput) Read(reader io.Reader) Output {
	return Output{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeOutput) Lower(value Output) RustBuffer {
	return LowerIntoRustBuffer[Output](c, value)
}

func (c FfiConverterTypeOutput) Write(writer io.Writer, value Output) {
	FfiConverterStringINSTANCE.Write(writer, value.Address)
	FfiConverterUint64INSTANCE.Write(writer, value.Amount)
	FfiConverterStringINSTANCE.Write(writer, value.Memo)
}

type FfiDestroyerTypeOutput struct{}

func (_ FfiDestroyerTypeOutput) Destroy(value Output) {
	value.Destroy()
}

type PartialTx struct {
	Height       uint32
	Txid         []byte
	Inputs       []Utxo
	Outputs      []Output
	Fee          uint64
	Sighashes    [][]byte
	ExpiryHeight uint32
	Version      uint32
}

func (r *PartialTx) Destroy() {
	FfiDestroyerUint32{}.Destroy(r.Height)
	FfiDestroyerBytes{}.Destroy(r.Txid)
	FfiDestroyerSequenceTypeUtxo{}.Destroy(r.Inputs)
	FfiDestroyerSequenceTypeOutput{}.Destroy(r.Outputs)
	FfiDestroyerUint64{}.Destroy(r.Fee)
	FfiDestroyerSequenceBytes{}.Destroy(r.Sighashes)
	FfiDestroyerUint32{}.Destroy(r.ExpiryHeight)
	FfiDestroyerUint32{}.Destroy(r.Version)
}

type FfiConverterTypePartialTx struct{}

var FfiConverterTypePartialTxINSTANCE = FfiConverterTypePartialTx{}

func (c FfiConverterTypePartialTx) Lift(rb RustBufferI) PartialTx {
	return LiftFromRustBuffer[PartialTx](c, rb)
}

func (c FfiConverterTypePartialTx) Read(reader io.Reader) PartialTx {
	return PartialTx{
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterBytesINSTANCE.Read(reader),
		FfiConverterSequenceTypeUTXOINSTANCE.Read(reader),
		FfiConverterSequenceTypeOutputINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
		FfiConverterSequenceBytesINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
	}
}

func (c FfiConverterTypePartialTx) Lower(value PartialTx) RustBuffer {
	return LowerIntoRustBuffer[PartialTx](c, value)
}

func (c FfiConverterTypePartialTx) Write(writer io.Writer, value PartialTx) {
	FfiConverterUint32INSTANCE.Write(writer, value.Height)
	FfiConverterBytesINSTANCE.Write(writer, value.Txid)
	FfiConverterSequenceTypeUTXOINSTANCE.Write(writer, value.Inputs)
	FfiConverterSequenceTypeOutputINSTANCE.Write(writer, value.Outputs)
	FfiConverterUint64INSTANCE.Write(writer, value.Fee)
	FfiConverterSequenceBytesINSTANCE.Write(writer, value.Sighashes)
	FfiConverterUint32INSTANCE.Write(writer, value.ExpiryHeight)
	FfiConverterUint32INSTANCE.Write(writer, value.Version)
}

type FfiDestroyerTypePartialTx struct{}

func (_ FfiDestroyerTypePartialTx) Destroy(value PartialTx) {
	value.Destroy()
}

type Utxo struct {
	Txid   string
	Height uint32
	Vout   uint32
	Script string
	Value  uint64
}

func (r *Utxo) Destroy() {
	FfiDestroyerString{}.Destroy(r.Txid)
	FfiDestroyerUint32{}.Destroy(r.Height)
	FfiDestroyerUint32{}.Destroy(r.Vout)
	FfiDestroyerString{}.Destroy(r.Script)
	FfiDestroyerUint64{}.Destroy(r.Value)
}

type FfiConverterTypeUTXO struct{}

var FfiConverterTypeUTXOINSTANCE = FfiConverterTypeUTXO{}

func (c FfiConverterTypeUTXO) Lift(rb RustBufferI) Utxo {
	return LiftFromRustBuffer[Utxo](c, rb)
}

func (c FfiConverterTypeUTXO) Read(reader io.Reader) Utxo {
	return Utxo{
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterUint32INSTANCE.Read(reader),
		FfiConverterStringINSTANCE.Read(reader),
		FfiConverterUint64INSTANCE.Read(reader),
	}
}

func (c FfiConverterTypeUTXO) Lower(value Utxo) RustBuffer {
	return LowerIntoRustBuffer[Utxo](c, value)
}

func (c FfiConverterTypeUTXO) Write(writer io.Writer, value Utxo) {
	FfiConverterStringINSTANCE.Write(writer, value.Txid)
	FfiConverterUint32INSTANCE.Write(writer, value.Height)
	FfiConverterUint32INSTANCE.Write(writer, value.Vout)
	FfiConverterStringINSTANCE.Write(writer, value.Script)
	FfiConverterUint64INSTANCE.Write(writer, value.Value)
}

type FfiDestroyerTypeUtxo struct{}

func (_ FfiDestroyerTypeUtxo) Destroy(value Utxo) {
	value.Destroy()
}

type Network uint

const (
	NetworkMain    Network = 1
	NetworkRegtest Network = 2
	NetworkTest    Network = 3
)

type FfiConverterTypeNetwork struct{}

var FfiConverterTypeNetworkINSTANCE = FfiConverterTypeNetwork{}

func (c FfiConverterTypeNetwork) Lift(rb RustBufferI) Network {
	return LiftFromRustBuffer[Network](c, rb)
}

func (c FfiConverterTypeNetwork) Lower(value Network) RustBuffer {
	return LowerIntoRustBuffer[Network](c, value)
}
func (FfiConverterTypeNetwork) Read(reader io.Reader) Network {
	id := readInt32(reader)
	return Network(id)
}

func (FfiConverterTypeNetwork) Write(writer io.Writer, value Network) {
	writeInt32(writer, int32(value))
}

type FfiDestroyerTypeNetwork struct{}

func (_ FfiDestroyerTypeNetwork) Destroy(value Network) {
}

type ZecError struct {
	err error
}

func (err ZecError) Error() string {
	return fmt.Sprintf("ZecError: %s", err.err.Error())
}

func (err ZecError) Unwrap() error {
	return err.err
}

// Err* are used for checking error type with `errors.Is`
var ErrZecErrorGenericError = fmt.Errorf("ZecErrorGenericError")
var ErrZecErrorInvalidVaultPubkey = fmt.Errorf("ZecErrorInvalidVaultPubkey")
var ErrZecErrorInvalidAddress = fmt.Errorf("ZecErrorInvalidAddress")
var ErrZecErrorInitError = fmt.Errorf("ZecErrorInitError")
var ErrZecErrorInvalidMemo = fmt.Errorf("ZecErrorInvalidMemo")
var ErrZecErrorInvalidAmount = fmt.Errorf("ZecErrorInvalidAmount")

// Variant structs
type ZecErrorGenericError struct {
	message string
}

func NewZecErrorGenericError() *ZecError {
	return &ZecError{
		err: &ZecErrorGenericError{},
	}
}

func (err ZecErrorGenericError) Error() string {
	return fmt.Sprintf("GenericError: %s", err.message)
}

func (self ZecErrorGenericError) Is(target error) bool {
	return target == ErrZecErrorGenericError
}

type ZecErrorInvalidVaultPubkey struct {
	message string
}

func NewZecErrorInvalidVaultPubkey() *ZecError {
	return &ZecError{
		err: &ZecErrorInvalidVaultPubkey{},
	}
}

func (err ZecErrorInvalidVaultPubkey) Error() string {
	return fmt.Sprintf("InvalidVaultPubkey: %s", err.message)
}

func (self ZecErrorInvalidVaultPubkey) Is(target error) bool {
	return target == ErrZecErrorInvalidVaultPubkey
}

type ZecErrorInvalidAddress struct {
	message string
}

func NewZecErrorInvalidAddress() *ZecError {
	return &ZecError{
		err: &ZecErrorInvalidAddress{},
	}
}

func (err ZecErrorInvalidAddress) Error() string {
	return fmt.Sprintf("InvalidAddress: %s", err.message)
}

func (self ZecErrorInvalidAddress) Is(target error) bool {
	return target == ErrZecErrorInvalidAddress
}

type ZecErrorInitError struct {
	message string
}

func NewZecErrorInitError() *ZecError {
	return &ZecError{
		err: &ZecErrorInitError{},
	}
}

func (err ZecErrorInitError) Error() string {
	return fmt.Sprintf("InitError: %s", err.message)
}

func (self ZecErrorInitError) Is(target error) bool {
	return target == ErrZecErrorInitError
}

type ZecErrorInvalidMemo struct {
	message string
}

func NewZecErrorInvalidMemo() *ZecError {
	return &ZecError{
		err: &ZecErrorInvalidMemo{},
	}
}

func (err ZecErrorInvalidMemo) Error() string {
	return fmt.Sprintf("InvalidMemo: %s", err.message)
}

func (self ZecErrorInvalidMemo) Is(target error) bool {
	return target == ErrZecErrorInvalidMemo
}

type ZecErrorInvalidAmount struct {
	message string
}

func NewZecErrorInvalidAmount() *ZecError {
	return &ZecError{
		err: &ZecErrorInvalidAmount{},
	}
}

func (err ZecErrorInvalidAmount) Error() string {
	return fmt.Sprintf("InvalidAmount: %s", err.message)
}

func (self ZecErrorInvalidAmount) Is(target error) bool {
	return target == ErrZecErrorInvalidAmount
}

type FfiConverterTypeZecError struct{}

var FfiConverterTypeZecErrorINSTANCE = FfiConverterTypeZecError{}

func (c FfiConverterTypeZecError) Lift(eb RustBufferI) error {
	return LiftFromRustBuffer[*ZecError](c, eb)
}

func (c FfiConverterTypeZecError) Lower(value *ZecError) RustBuffer {
	return LowerIntoRustBuffer[*ZecError](c, value)
}

func (c FfiConverterTypeZecError) Read(reader io.Reader) *ZecError {
	errorID := readUint32(reader)

	message := FfiConverterStringINSTANCE.Read(reader)
	switch errorID {
	case 1:
		return &ZecError{&ZecErrorGenericError{message}}
	case 2:
		return &ZecError{&ZecErrorInvalidVaultPubkey{message}}
	case 3:
		return &ZecError{&ZecErrorInvalidAddress{message}}
	case 4:
		return &ZecError{&ZecErrorInitError{message}}
	case 5:
		return &ZecError{&ZecErrorInvalidMemo{message}}
	case 6:
		return &ZecError{&ZecErrorInvalidAmount{message}}
	default:
		panic(fmt.Sprintf("Unknown error code %d in FfiConverterTypeZecError.Read()", errorID))
	}

}

func (c FfiConverterTypeZecError) Write(writer io.Writer, value *ZecError) {
	switch variantValue := value.err.(type) {
	case *ZecErrorGenericError:
		writeInt32(writer, 1)
	case *ZecErrorInvalidVaultPubkey:
		writeInt32(writer, 2)
	case *ZecErrorInvalidAddress:
		writeInt32(writer, 3)
	case *ZecErrorInitError:
		writeInt32(writer, 4)
	case *ZecErrorInvalidMemo:
		writeInt32(writer, 5)
	case *ZecErrorInvalidAmount:
		writeInt32(writer, 6)
	default:
		_ = variantValue
		panic(fmt.Sprintf("invalid error value `%v` in FfiConverterTypeZecError.Write", value))
	}
}

type FfiConverterSequenceBytes struct{}

var FfiConverterSequenceBytesINSTANCE = FfiConverterSequenceBytes{}

func (c FfiConverterSequenceBytes) Lift(rb RustBufferI) [][]byte {
	return LiftFromRustBuffer[[][]byte](c, rb)
}

func (c FfiConverterSequenceBytes) Read(reader io.Reader) [][]byte {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([][]byte, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterBytesINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceBytes) Lower(value [][]byte) RustBuffer {
	return LowerIntoRustBuffer[[][]byte](c, value)
}

func (c FfiConverterSequenceBytes) Write(writer io.Writer, value [][]byte) {
	if len(value) > math.MaxInt32 {
		panic("[][]byte is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterBytesINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceBytes struct{}

func (FfiDestroyerSequenceBytes) Destroy(sequence [][]byte) {
	for _, value := range sequence {
		FfiDestroyerBytes{}.Destroy(value)
	}
}

type FfiConverterSequenceTypeOutput struct{}

var FfiConverterSequenceTypeOutputINSTANCE = FfiConverterSequenceTypeOutput{}

func (c FfiConverterSequenceTypeOutput) Lift(rb RustBufferI) []Output {
	return LiftFromRustBuffer[[]Output](c, rb)
}

func (c FfiConverterSequenceTypeOutput) Read(reader io.Reader) []Output {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Output, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTypeOutputINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTypeOutput) Lower(value []Output) RustBuffer {
	return LowerIntoRustBuffer[[]Output](c, value)
}

func (c FfiConverterSequenceTypeOutput) Write(writer io.Writer, value []Output) {
	if len(value) > math.MaxInt32 {
		panic("[]Output is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTypeOutputINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTypeOutput struct{}

func (FfiDestroyerSequenceTypeOutput) Destroy(sequence []Output) {
	for _, value := range sequence {
		FfiDestroyerTypeOutput{}.Destroy(value)
	}
}

type FfiConverterSequenceTypeUTXO struct{}

var FfiConverterSequenceTypeUTXOINSTANCE = FfiConverterSequenceTypeUTXO{}

func (c FfiConverterSequenceTypeUTXO) Lift(rb RustBufferI) []Utxo {
	return LiftFromRustBuffer[[]Utxo](c, rb)
}

func (c FfiConverterSequenceTypeUTXO) Read(reader io.Reader) []Utxo {
	length := readInt32(reader)
	if length == 0 {
		return nil
	}
	result := make([]Utxo, 0, length)
	for i := int32(0); i < length; i++ {
		result = append(result, FfiConverterTypeUTXOINSTANCE.Read(reader))
	}
	return result
}

func (c FfiConverterSequenceTypeUTXO) Lower(value []Utxo) RustBuffer {
	return LowerIntoRustBuffer[[]Utxo](c, value)
}

func (c FfiConverterSequenceTypeUTXO) Write(writer io.Writer, value []Utxo) {
	if len(value) > math.MaxInt32 {
		panic("[]Utxo is too large to fit into Int32")
	}

	writeInt32(writer, int32(len(value)))
	for _, item := range value {
		FfiConverterTypeUTXOINSTANCE.Write(writer, item)
	}
}

type FfiDestroyerSequenceTypeUtxo struct{}

func (FfiDestroyerSequenceTypeUtxo) Destroy(sequence []Utxo) {
	for _, value := range sequence {
		FfiDestroyerTypeUtxo{}.Destroy(value)
	}
}

func ApplySignatures(vault []byte, ptx PartialTx, signatures [][]byte, network Network) ([]byte, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeZecError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_zec_fn_func_apply_signatures(FfiConverterBytesINSTANCE.Lower(vault), FfiConverterTypePartialTxINSTANCE.Lower(ptx), FfiConverterSequenceBytesINSTANCE.Lower(signatures), FfiConverterTypeNetworkINSTANCE.Lower(network), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func BuildPtx(vault []byte, ptx PartialTx, network Network) (PartialTx, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeZecError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_zec_fn_func_build_ptx(FfiConverterBytesINSTANCE.Lower(vault), FfiConverterTypePartialTxINSTANCE.Lower(ptx), FfiConverterTypeNetworkINSTANCE.Lower(network), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue PartialTx
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterTypePartialTxINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func ComputeTxid(vault []byte, ptx PartialTx, network Network) (string, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeZecError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_zec_fn_func_compute_txid(FfiConverterBytesINSTANCE.Lower(vault), FfiConverterTypePartialTxINSTANCE.Lower(ptx), FfiConverterTypeNetworkINSTANCE.Lower(network), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue string
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterStringINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func GetOvk(vault []byte) ([]byte, error) {
	_uniffiRV, _uniffiErr := rustCallWithError(FfiConverterTypeZecError{}, func(_uniffiStatus *C.RustCallStatus) RustBufferI {
		return C.uniffi_zec_fn_func_get_ovk(FfiConverterBytesINSTANCE.Lower(vault), _uniffiStatus)
	})
	if _uniffiErr != nil {
		var _uniffiDefaultValue []byte
		return _uniffiDefaultValue, _uniffiErr
	} else {
		return FfiConverterBytesINSTANCE.Lift(_uniffiRV), _uniffiErr
	}
}

func InitZec() error {
	_, _uniffiErr := rustCallWithError(FfiConverterTypeZecError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_zec_fn_func_init_zec(_uniffiStatus)
		return false
	})
	return _uniffiErr
}

func ValidateAddress(address string, network Network) error {
	_, _uniffiErr := rustCallWithError(FfiConverterTypeZecError{}, func(_uniffiStatus *C.RustCallStatus) bool {
		C.uniffi_zec_fn_func_validate_address(FfiConverterStringINSTANCE.Lower(address), FfiConverterTypeNetworkINSTANCE.Lower(network), _uniffiStatus)
		return false
	})
	return _uniffiErr
}
