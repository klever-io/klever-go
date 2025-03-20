#include "../mxvm/context.h"
#include "../mxvm/util.h"


byte funcName[] = "get_owner_address_test";

void execute_on_dest_context(byte *call_address, byte *function, int funcLen){
	byte value[32] = {0};
	int arguments_length = 0;
	byte arguments[0];
		
	int result = executeOnDestContext(getGasLeft(), call_address, value, function, funcLen, 0, arguments_length, arguments);
	CHECK_RESULT_CODE(result, 0);
}

int isZero(byte *address, int length) {
    for (int i = 0; i < length; i++) {
        if (address[i] != 0) {
            return 0;
        }
    }
    return 1;
}

void get_owner_address_test() {
	byte address[32] = {0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0};
	getOwnerAddress(address);
    REQUIRE(!isZero(address, 32),"zero address!");
	finish(address, 32);
}

void delete_contract(byte *address, long long gasLimit) {
    int numArguments = 0;
    byte argumentsLengthOffset[0];
    byte dataOffset[0];
    deleteContract(address, gasLimit, numArguments, argumentsLengthOffset, dataOffset);
}

void delete_contract_half_gas() {
    byte delete_address[32] = {0};
    getArgument(0, delete_address);
    execute_on_dest_context(delete_address, funcName, sizeof(funcName)-1);
    // use half of the gas provided
    long long gasLeft = getGasLeft() / 2;
    delete_contract(delete_address, gasLeft);
    execute_on_dest_context(delete_address, funcName, sizeof(funcName)-1);
}

void delete_contract_less_gas() {
    byte delete_address[32] = {0};
    getArgument(0, delete_address);
    execute_on_dest_context(delete_address, funcName, sizeof(funcName)-1);
    // provide less gas than needed
    delete_contract(delete_address, 100);
    execute_on_dest_context(delete_address, funcName, sizeof(funcName)-1);
}

void delete_contract_full_gas() {
    byte delete_address[32] = {0};
    getArgument(0, delete_address);
    execute_on_dest_context(delete_address, funcName, sizeof(funcName)-1);
    // use all the gas provided
    long long gasLeft = getGasLeft();
    delete_contract(delete_address, gasLeft);
    execute_on_dest_context(delete_address, funcName, sizeof(funcName)-1);
}