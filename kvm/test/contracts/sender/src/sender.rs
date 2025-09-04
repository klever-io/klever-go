#![no_std]

use klever_sc::require;

/// An empty contract. To be used as a template when starting a new contract from scratch.
#[klever_sc::contract]
pub trait Sender {
    #[init]
    fn init(&self) {}

    #[upgrade]
    fn upgrade(&self) {}

    
    #[payable("*")]
    #[endpoint(sendToAddress)]
    fn send_to_test(&self, to: ManagedAddress) {
        let (token, value) = self.call_value().klv_or_single_fungible_kda();

        require!(token.is_valid_kda_identifier(), "Only KDA transfers are allowed");

        self.send().direct_kda(&to, &token, 0, &value);
    }
}
