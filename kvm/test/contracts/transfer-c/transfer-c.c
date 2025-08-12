#include "../mxvm/context.h"
#include "../mxvm/bigInt.h"
#include "../mxvm/test_utils.h"
#include "../mxvm/transfer.h"

void transfer() {
    // Create managed buffers for the transfer
    int dstHandle = mBufferNew();
    int tokenTransfersHandle = mBufferNew();
    int functionHandle = mBufferNew();
    int argumentsHandle = mBufferNew();
    
    // Check if buffer creation was successful
    if (dstHandle < 0 || tokenTransfersHandle < 0 || functionHandle < 0 || argumentsHandle < 0) {
        finishResult(-1); // Buffer creation failed
        return;
    }
    
    // Set destination address to userAccount (32-byte address)
    byte destinationAddr[] = "\0\0\0\0\0\0\0\0\x0F\x0F" "userAccount...............";
    mBufferSetBytes(dstHandle, destinationAddr, 32);
    
    // Create token identifier buffer for KLV
    int klvTokenHandle = mBufferNew();
    byte klvToken[] = "KLV";
    mBufferSetBytes(klvTokenHandle, klvToken, 3);
    
    // Create BigInt handle for 1 KLV (1000000)
    int valueHandle = bigIntNew(1000000);
    
    // Create the transfer data structure (16 bytes per transfer)
    // Format: [tokenIdHandle:4][nonce:8][valueHandle:4]
    byte transferData[16];
    
    // Token handle (4 bytes, big endian)
    transferData[0] = (byte)((klvTokenHandle >> 24) & 0xFF);
    transferData[1] = (byte)((klvTokenHandle >> 16) & 0xFF);
    transferData[2] = (byte)((klvTokenHandle >> 8) & 0xFF);
    transferData[3] = (byte)(klvTokenHandle & 0xFF);
    
    // Nonce (8 bytes, zero for KLV fungible token)
    for (int i = 4; i < 12; i++) {
        transferData[i] = 0;
    }
    
    // Value handle (4 bytes, big endian)
    transferData[12] = (byte)((valueHandle >> 24) & 0xFF);
    transferData[13] = (byte)((valueHandle >> 16) & 0xFF);
    transferData[14] = (byte)((valueHandle >> 8) & 0xFF);
    transferData[15] = (byte)(valueHandle & 0xFF);
    
    mBufferSetBytes(tokenTransfersHandle, transferData, 16);
    
    // Empty function name (no function call, just transfer)
    byte emptyFunction[] = "";
    mBufferSetBytes(functionHandle, emptyFunction, 0);
    
    // Empty arguments
    byte emptyArgs[] = "";
    mBufferSetBytes(argumentsHandle, emptyArgs, 0);
    
    // Execute the KLV transfer with sufficient gas
    int result = managedMultiTransferKDANFTExecute(
        dstHandle,
        tokenTransfersHandle,
        50000,  // Sufficient gas for transfer
        functionHandle,
        argumentsHandle
    );
    
    finishResult(result);
}

void transfer_with_function() {
    int dstHandle = mBufferNew();

    byte destinationAddr[] = "\0\0\0\0\0\0\0\0\x0F\x0F" "userAccount...............";

    int result = performKLVTransfer(destinationAddr, 1000000, 50000);
    
    finishResult(result);
}