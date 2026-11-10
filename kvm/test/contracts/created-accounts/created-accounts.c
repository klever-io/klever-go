// created-accounts measures the gas a contract pays for each account that a builtin it calls
// creates, against plain KLV transfers to fresh addresses (KLC-2938).
//
// Build from this directory (clang and wasm-ld 19):
//   for f in created-accounts.c ../mxvm/cbuiltins.c; do
//     clang --target=wasm32-unknown-unknown -O3 -ffreestanding -fno-builtin -nostdlib -c $f -o $(basename $f .c).o
//   done
//   wasm-ld --no-entry --strip-all --allow-undefined $(sed 's/^/--export=/' created-accounts.export) \
//     created-accounts.o cbuiltins.o -o output/created-accounts.wasm
#include "../mxvm/context.h"
#include "../mxvm/transfer.h"

#define MAX_ARGS 16
#define MAX_ARG_BYTES 4096
#define NAME_BYTES 64
// kept by this contract for its own work after each builtin returns
#define GAS_RESERVE 100000

byte zero[32] = {0};
byte self[32];
byte name[NAME_BYTES];
byte data[MAX_ARG_BYTES];
int lengths[MAX_ARGS];

void init() {}

void fail(char *message, int length) {
	signalError((byte *)message, length);
}

// forward(builtin, gas, n, arg1..argn, builtin, gas, n, ...) calls each named builtin on this
// contract, in order, with its n arguments, so the builtins run with this contract as their caller.
// A builtin gets gas, or with gas 0 all the gas left but GAS_RESERVE. One call can repeat a sequence,
// which prices a repetition without the cost of the call itself.
void forward() {
	int total = getNumArguments();
	getSCAddress(self);

	int i = 0;
	while (i < total) {
		if (i + 2 >= total || getArgumentLength(i) > NAME_BYTES) {
			fail("bad arguments", 13);
			return;
		}
		int nameLength = getArgument(i, name);
		long long gas = int64getArgument(i + 1);
		long long n = int64getArgument(i + 2);
		i += 3;
		if (gas < 0 || n < 0 || n > MAX_ARGS || i + n > total) {
			fail("bad arguments", 13);
			return;
		}

		int offset = 0;
		for (int j = 0; j < n; j++, i++) {
			int length = getArgumentLength(i);
			if (offset + length > MAX_ARG_BYTES) {
				fail("arguments too long", 18);
				return;
			}
			getArgument(i, data + offset);
			lengths[j] = length;
			offset += length;
		}

		if (gas == 0) {
			long long left = getGasLeft();
			if (left < GAS_RESERVE) {
				fail("out of gas", 10);
				return;
			}
			gas = left - GAS_RESERVE;
		}
		if (executeOnDestContext(gas, self, zero, name, nameLength, (int)n, (byte *)lengths, data) != 0) {
			fail("builtin failed", 14);
			return;
		}
	}
}

// sendKLV(amount, address...) sends amount KLV to each address.
void sendKLV() {
	int n = getNumArguments();
	long long amount = int64getArgument(0);
	byte destination[32];
	for (int i = 1; i < n; i++) {
		if (getArgumentLength(i) != 32) {
			fail("bad address", 11);
			return;
		}
		getArgument(i, destination);
		if (performKLVTransfer(destination, amount, 0) != 0) {
			fail("transfer failed", 15);
			return;
		}
	}
}
