#include "../../mxvm/context.h"
#include "../../mxvm/test_utils.h"
#include "../../mxvm/args.h"

byte executeValue[] = {0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0};
byte self[32] = "\0\0\0\0\0\0\0\0\x0f\x0f" "parentSC..............";
byte vaultSC[] = "\0\0\0\0\0\0\0\0\x0F\x0F" "vaultSC...............";
byte KDATransfer[] = "KDATransfer";

void basic_transfer() {
	byte tokenName[265] = {0};
	int tokenNameLen = getKDATokenName(tokenName);

	byte kdaValue[32] = {0};
	int kdaValueLen = getKDAValue(kdaValue);

	kdaValue[31] -= 1;

	BinaryArgs args = NewBinaryArgs();

	int lastArg = 0;
	lastArg = AddBinaryArg(&args, tokenName, tokenNameLen);
	lastArg = AddBinaryArg(&args, kdaValue, kdaValueLen);
	TrimLeftZeros(&args, lastArg);

	byte arguments[100];
	int argsLen = SerializeBinaryArgs(&args, arguments);

	int result = executeOnDestContext(
			1000000,
			self,
			executeValue,
			KDATransfer,
			sizeof KDATransfer - 1,
			args.numArgs,
		  (byte*)args.lengthsAsI32,
			args.serialized
	);
}
