#ifndef _TRANSFER_H_
#define _TRANSFER_H_

#include "types.h"
#include "bigInt.h"

int performKLVTransfer(byte* destination, long long amount, long long gasLimit) {
    // Create managed buffers for the transfer
    int dstHandle = mBufferNew();
    int tokenTransfersHandle = mBufferNew();
    int functionHandle = mBufferNew();
    int argumentsHandle = mBufferNew();
    
    // Set destination address (assume 32-byte address)
    mBufferSetBytes(dstHandle, destination, 32);
    
    // Create token identifier buffer for KLV
    int klvTokenHandle = mBufferNew();
    byte klvToken[] = "KLV";
    mBufferSetBytes(klvTokenHandle, klvToken, 3);
    
    // Create BigInt handle for the transfer amount
    int valueHandle = bigIntNew(amount);
    
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
    
    byte emptyFunction[] = "";
    mBufferSetBytes(functionHandle, emptyFunction, 0);
    
    byte emptyArgs[] = "";
    mBufferSetBytes(argumentsHandle, emptyArgs, 0);
    
    int result = managedMultiTransferKDANFTExecute(
        dstHandle,
        tokenTransfersHandle,
        gasLimit,
        functionHandle,
        argumentsHandle
    );
    
    return result;
}

#endif
