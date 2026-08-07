#include "../mxvm/context.h"

// Child contract used by the managed-exec-on-dest-context aliasing PoC.
// It exposes plain storage read/write endpoints and is deployed twice: once
// as the real callee, once again as an unrelated "victim" contract that
// should never be touched by a call targeting the callee.
//
// Deliberately does NOT include ../mxvm/test_utils.h: buildWasmC.sh links
// with `wasm-ld --export-all`, so any non-static helper pulled in from that
// header (most take parameters / return values) would also get exported and
// trip the node's ValidateFunctionArities check (every exported function
// must be void->void), causing a client-side ContractInvalid on deploy.

byte writtenMsg[] = "written";

void init() {}

void writeStorage() {
	int numArgs = getNumArguments();
	if (numArgs != 2) {
		byte message[] = "wrong number of arguments";
		signalError(message, 25);
	}

	byte key[64];
	int keyLen = getArgumentLength(0);
	if (keyLen > 64) {
		byte message[] = "key argument too long";
		signalError(message, 22);
	}
	getArgument(0, key);

	byte value[64];
	int valueLen = getArgumentLength(1);
	if (valueLen > 64) {
		byte message[] = "value argument too long";
		signalError(message, 24);
	}
	getArgument(1, value);

	storageStore(key, keyLen, value, valueLen);

	finish(writtenMsg, 7);
}
