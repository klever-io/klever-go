#![no_std]

use klever_sc::imports::*;
mod sender_proxy;

/// An empty contract. To be used as a template when starting a new contract from scratch.
#[klever_sc::contract]
pub trait Example {
    #[init]
    fn init(&self) {}

    #[upgrade]
    fn upgrade(&self) {}

    
    #[payable("*")]
    #[endpoint(proxySendToAddress)]
    fn send_to_address(&self, call_contract: ManagedAddress ,to: ManagedAddress) {
        let (token_identifier, value) = self.call_value().klv_or_single_fungible_kda();

        require!(token_identifier.is_valid_kda_identifier(), "Only KDA transfers are allowed");

        self.tx()
            .to(call_contract)
            .typed(sender_proxy::SenderProxy)
            .send_to_address(to)
            .single_kda(&token_identifier, 0, &value)
            .sync_call()
    }
}
