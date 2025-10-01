use maya_router::{MayaRouterDepositEvent, MayaRouterDirectDepositEvent, MayaRouterWithdrawEvent};
use radix_engine::system::system_type_checker::TypeCheckError;
use scrypto_compiler::ScryptoCompiler;
use scrypto_test::prelude::LedgerSimulatorBuilder;
use scrypto_test::prelude::*;
use strum::IntoEnumIterator;
use strum_macros::EnumIter;

enum KeyType {
    Ed25519,
    Secp256k1,
}

struct Account {
    _public_key: PublicKey,
    _private_key: PrivateKey,
    address: ComponentAddress,
    badge: NonFungibleGlobalId,
}

impl Account {
    fn new(ledger: &mut DefaultLedgerSimulator, key_type: KeyType) -> Self {
        let account: (PublicKey, PrivateKey, ComponentAddress) = match key_type {
            KeyType::Ed25519 => {
                let (public_key, private_key, address) = ledger.new_ed25519_virtual_account();
                (public_key.into(), private_key.into(), address)
            }
            KeyType::Secp256k1 => {
                let (public_key, private_key, address) = ledger.new_virtual_account();
                (public_key.into(), private_key.into(), address)
            }
        };

        let badge = NonFungibleGlobalId::from_public_key(&account.0);

        Account {
            _public_key: account.0,
            _private_key: account.1,
            address: account.2,
            badge,
        }
    }

    fn new_not_initialized(ledger: &mut DefaultLedgerSimulator, key_type: KeyType) -> Self {
        let account: (PublicKey, PrivateKey) = match key_type {
            KeyType::Ed25519 => {
                let (public_key, private_key) = ledger.new_ed25519_key_pair();
                (public_key.into(), private_key.into())
            }
            KeyType::Secp256k1 => {
                let (public_key, private_key) = ledger.new_key_pair();
                (public_key.into(), private_key.into())
            }
        };
        let address = ComponentAddress::virtual_account_from_public_key(&account.0.clone());

        let badge = NonFungibleGlobalId::from_public_key(&account.0);

        Account {
            _public_key: account.0,
            _private_key: account.1,
            address,
            badge,
        }
    }
}

struct MayaRouterSimulator {
    pub ledger: DefaultLedgerSimulator,
    pub component_address: ComponentAddress,
    pub _owner: Account,
    pub asgard_vault_1: Account,
    pub asgard_vault_2: Account,
    pub swapper: Account,
    pub resources: IndexMap<String, ResourceAddress>,
}

impl MayaRouterSimulator {
    const BLUEPRINT_NAME: &'static str = "MayaRouter";

    pub fn create_component(ledger: &mut DefaultLedgerSimulator) -> TransactionReceipt {
        let mut compiler = ScryptoCompiler::builder()
            .manifest_path(this_package!())
            .build()
            .unwrap();
        let artifacts = compiler.compile().unwrap().remove(0);

        let package_address = ledger
            .publish_package_simple((artifacts.wasm.content, artifacts.package_definition.content));
        ledger.execute_manifest(
            ManifestBuilder::new()
                .lock_fee_from_faucet()
                .call_function(
                    package_address,
                    Self::BLUEPRINT_NAME,
                    "instantiate",
                    manifest_args!(),
                )
                .build(),
            vec![],
        )
    }

    pub fn new() -> Self {
        let mut ledger = LedgerSimulatorBuilder::new().build();
        // Owner account
        let owner = Account::new(&mut ledger, KeyType::Ed25519);

        // We are using not initialized accounts. They will be initialized once someone
        // will deposit some resources to them.
        let asgard_vault_1 = Account::new_not_initialized(&mut ledger, KeyType::Secp256k1);
        let asgard_vault_2 = Account::new_not_initialized(&mut ledger, KeyType::Ed25519);
        let swapper = Account::new(&mut ledger, KeyType::Secp256k1);

        let mut resources = indexmap!();
        resources.insert("XRD".to_string(), XRD);
        for symbol in ["USDT", "ETH"] {
            resources.insert(
                symbol.to_string(),
                ledger.create_fungible_resource(dec!(20000), 18, swapper.address),
            );
        }
        for symbol in ["BTC", "SOL"] {
            resources.insert(
                symbol.to_string(),
                ledger.create_fungible_resource(dec!(20000), 18, owner.address),
            );
        }

        let receipt = Self::create_component(&mut ledger);
        let component_address = receipt.expect_commit_success().new_component_addresses()[0];
        Self {
            ledger,
            component_address,
            _owner: owner,
            asgard_vault_1,
            asgard_vault_2,
            swapper,
            resources,
        }
    }

    pub fn get_account_locker(&mut self) -> Option<ComponentAddress> {
        for component in self.ledger.find_all_components() {
            if component.as_node_id().entity_type() == Some(EntityType::GlobalAccountLocker) {
                return Some(component);
            }
        }
        None
    }

    pub fn set_only_swapper_can_deposit(&mut self) {
        let badge = ResourceOrNonFungible::NonFungible(self.swapper.badge.clone());
        self.ledger.execute_manifest(
            ManifestBuilder::new()
                .lock_fee_from_faucet()
                .call_method(
                    self.swapper.address,
                    ACCOUNT_SET_DEFAULT_DEPOSIT_RULE_IDENT,
                    AccountSetDefaultDepositRuleInput {
                        default: DefaultDepositRule::Reject,
                    },
                )
                .call_method(
                    self.swapper.address,
                    ACCOUNT_ADD_AUTHORIZED_DEPOSITOR,
                    AccountAddAuthorizedDepositorInput {
                        badge: badge.clone(),
                    },
                )
                .build(),
            vec![self.swapper.badge.clone()],
        );
    }

    pub fn get_swapper_balance(&mut self, asset: ResourceAddress) -> Decimal {
        self.ledger
            .get_component_balance(self.swapper.address, asset)
    }

    pub fn deposit(
        &mut self,
        vault_address: ComponentAddress,
        from: ComponentAddress,
        badge: NonFungibleGlobalId,
        asset: ResourceAddress,
        amount: Decimal,
        memo: &str,
    ) -> TransactionReceipt {
        let manifest = ManifestBuilder::new()
            .lock_fee_from_faucet()
            .withdraw_from_account(from, asset, amount)
            .take_all_from_worktop(asset, "bucket")
            .with_bucket("bucket", |builder, bucket| {
                builder.call_method(
                    self.component_address,
                    "user_deposit",
                    manifest_args!(from, vault_address, bucket, memo.to_string()),
                )
            })
            .build();
        self.ledger.execute_manifest(manifest, vec![badge])
    }

    pub fn transfer_out(
        &mut self,
        vault_address: ComponentAddress,
        badge: NonFungibleGlobalId,
        to: ComponentAddress,
        asset: ResourceAddress,
        amount: Decimal,
        memo: &str,
        fee_to_lock: Decimal,
    ) -> TransactionReceipt {
        let manifest = ManifestBuilder::new()
            .call_method(
                self.component_address,
                "lock_fee",
                manifest_args!(vault_address, fee_to_lock),
            )
            .call_method(
                self.component_address,
                "withdraw",
                manifest_args!(
                    vault_address,
                    asset,
                    to,
                    Option::<()>::None,
                    amount,
                    memo.to_string()
                ),
            )
            .take_all_from_worktop(asset, "asset")
            .call_method_with_name_lookup(self.component_address, "transfer", |lookup| {
                (to, lookup.bucket("asset"))
            })
            .build();
        self.ledger.execute_manifest(manifest, vec![badge])
    }

    pub fn transfer_between_vaults(
        &mut self,
        from_vault_address: ComponentAddress,
        badge: NonFungibleGlobalId,
        to_vault_address: ComponentAddress,
        asset: ResourceAddress,
        amount: Decimal,
        memo: &str,
        fee_to_lock: Decimal,
    ) -> TransactionReceipt {
        let manifest = ManifestBuilder::new()
            .call_method(
                self.component_address,
                "lock_fee",
                manifest_args!(from_vault_address, fee_to_lock),
            )
            .call_method(
                self.component_address,
                "withdraw",
                manifest_args!(
                    from_vault_address,
                    asset,
                    to_vault_address,
                    Option::<()>::None,
                    amount,
                    memo.to_string()
                ),
            )
            .take_all_from_worktop(asset, "asset")
            .call_method_with_name_lookup(self.component_address, "direct_deposit", |lookup| {
                (to_vault_address, lookup.bucket("asset"))
            })
            .build();
        self.ledger.execute_manifest(manifest, vec![badge])
    }
}

#[test]
fn maya_router_deposit_and_transfer_out_success() {
    // Arrange
    let mut maya_router = MayaRouterSimulator::new();

    // Act
    let swap_memo = "SWAP:MAYA.CACAO";
    let tx_out_memo = "OUT:";

    // Perform Swap
    // Act
    let receipt = maya_router.deposit(
        maya_router.asgard_vault_1.address,
        maya_router.swapper.address,
        maya_router.swapper.badge.clone(),
        XRD,
        dec!(1000),
        swap_memo,
    );

    // Assert
    let result = receipt.expect_commit_success();
    let events = result.application_events.as_slice();

    let event_data = events
        .iter()
        .find(|(type_identifier, _)| {
            type_identifier.eq(&EventTypeIdentifier(
                Emitter::Method(maya_router.component_address.into_node_id(), ModuleId::Main),
                "MayaRouterDepositEvent".to_string(),
            ))
        })
        .map(|(_, data)| data)
        .expect("MayaRouterDepositEvent not found");

    assert_eq!(
        scrypto_decode::<MayaRouterDepositEvent>(&event_data).unwrap(),
        MayaRouterDepositEvent {
            sender: maya_router.swapper.address,
            vault_address: maya_router.asgard_vault_1.address,
            resource_address: XRD,
            amount: dec!(1000),
            memo: swap_memo.to_string(),
        }
    );

    #[derive(EnumIter)]
    enum TransferOut {
        NoLockerUsed,
        LockerUsed,
    }

    for variant in TransferOut::iter() {
        // Perform Send
        // Arrange
        let balance = maya_router.get_swapper_balance(XRD);

        // Act
        let receipt = maya_router.transfer_out(
            maya_router.asgard_vault_1.address,
            maya_router.asgard_vault_1.badge.clone(),
            maya_router.swapper.address,
            XRD,
            dec!(100),
            tx_out_memo,
            dec!(10),
        );

        // Assert
        let result = receipt.expect_commit_success();
        let events = result.application_events.as_slice();

        let event_data = events
            .iter()
            .find(|(type_identifier, _)| {
                type_identifier.eq(&EventTypeIdentifier(
                    Emitter::Method(maya_router.component_address.into_node_id(), ModuleId::Main),
                    "MayaRouterWithdrawEvent".to_string(),
                ))
            })
            .map(|(_, data)| data)
            .expect("MayaRouterWithdrawEvent not found");

        assert_eq!(
            scrypto_decode::<MayaRouterWithdrawEvent>(&event_data).unwrap(),
            MayaRouterWithdrawEvent {
                resource_address: XRD,
                amount: dec!(100),
                memo: tx_out_memo.to_string(),
                vault_address: maya_router.asgard_vault_1.address,
                intended_recipient: maya_router.swapper.address,
                aggregator: None
            }
        );

        match variant {
            TransferOut::NoLockerUsed => {
                assert_eq!(balance + dec!(100), maya_router.get_swapper_balance(XRD));

                maya_router.set_only_swapper_can_deposit();
            }
            TransferOut::LockerUsed => {
                let locker = maya_router.get_account_locker().unwrap();

                let manifest = ManifestBuilder::new()
                    .lock_fee_from_faucet()
                    .call_method(
                        locker,
                        "claim",
                        manifest_args!(maya_router.swapper.address, XRD, dec!(100)),
                    )
                    .try_deposit_entire_worktop_or_abort(
                        maya_router.swapper.address,
                        Some(ResourceOrNonFungible::NonFungible(
                            maya_router.swapper.badge.clone(),
                        )),
                    )
                    .build();
                let receipt = maya_router
                    .ledger
                    .execute_manifest(manifest, vec![maya_router.swapper.badge.clone()]);
                receipt.expect_commit_success();

                assert_eq!(balance + dec!(100), maya_router.get_swapper_balance(XRD));
            }
        }
    }
}

#[test]
fn maya_router_deposit_sender_non_real_account() {
    // Arrange
    let mut maya_router = MayaRouterSimulator::new();

    // Act
    let swap_memo = "SWAP:MAYA.CACAO";

    // Act
    let manifest = ManifestBuilder::new()
        .lock_fee_from_faucet()
        .withdraw_from_account(maya_router.swapper.address, XRD, dec!(100))
        .take_all_from_worktop(XRD, "bucket")
        .with_bucket("bucket", |builder, bucket| {
            builder.call_method(
                maya_router.component_address,
                "user_deposit",
                manifest_args!(
                    maya_router.component_address, // non-account component address
                    maya_router.asgard_vault_1.address,
                    bucket,
                    swap_memo.to_string()
                ),
            )
        })
        .build();
    let receipt = maya_router
        .ledger
        .execute_manifest(manifest, vec![maya_router.swapper.badge]);

    // Assert
    receipt.expect_specific_failure(|e| {
        matches!(
            e,
            RuntimeError::SystemError(SystemError::TypeCheckError(
                TypeCheckError::BlueprintPayloadValidationError(..)
            ))
        )
    });
}

#[test]
fn maya_router_transfer_out_negative_fee_to_lock() {
    // Arrange
    let mut maya_router = MayaRouterSimulator::new();
    let fee_to_lock = dec!(-10);

    // Act
    let swap_memo = "SWAP:MAYA.CACAO";
    let tx_out_memo = "OUT:";

    // Perform Swap
    // Act
    let receipt = maya_router.deposit(
        maya_router.asgard_vault_1.address,
        maya_router.swapper.address,
        maya_router.swapper.badge.clone(),
        XRD,
        dec!(100),
        swap_memo,
    );

    // Assert
    receipt.expect_commit_success();

    // Perform Send of non-existing asset
    // Act
    let receipt = maya_router.transfer_out(
        maya_router.asgard_vault_1.address,
        maya_router.asgard_vault_1.badge.clone(),
        maya_router.swapper.address,
        XRD,
        dec!(50),
        tx_out_memo,
        fee_to_lock,
    );

    // Assert
    receipt.expect_specific_rejection(|e| match e {
        RejectionReason::ErrorBeforeLoanAndDeferredCostsRepaid(RuntimeError::ApplicationError(
            ApplicationError::PanicMessage(s),
        )) => format!("Negative fee to lock {:?} provided", fee_to_lock) == s.to_string(),
        _ => false,
    });
}

#[test]
fn maya_router_transfer_out_too_big_amount() {
    // Arrange
    let mut maya_router = MayaRouterSimulator::new();

    // Act
    let swap_memo = "SWAP:MAYA.CACAO";
    let tx_out_memo = "OUT:";

    // Perform Swap
    // Act
    let receipt = maya_router.deposit(
        maya_router.asgard_vault_1.address,
        maya_router.swapper.address,
        maya_router.swapper.badge.clone(),
        XRD,
        dec!(100),
        swap_memo,
    );

    // Assert
    receipt.expect_commit_success();

    // Perform Send of non-existing asset
    // Act
    let receipt = maya_router.transfer_out(
        maya_router.asgard_vault_1.address,
        maya_router.asgard_vault_1.badge.clone(),
        maya_router.swapper.address,
        XRD,
        dec!(91),
        tx_out_memo,
        dec!(10),
    );

    // Assert
    receipt.expect_specific_failure(|e| match e {
        RuntimeError::ApplicationError(ApplicationError::PanicMessage(s)) => {
            s.contains("balance") && s.contains("lower than taken amount")
        }
        _ => false,
    });
}

#[test]
fn maya_router_transfer_out_asset_not_available() {
    // Arrange
    let mut maya_router = MayaRouterSimulator::new();

    // Act
    let swap_memo = "SWAP:MAYA.CACAO";
    let tx_out_memo = "OUT:";
    let fee_to_lock = dec!(10);

    // Perform Swap
    // Act
    let receipt = maya_router.deposit(
        maya_router.asgard_vault_1.address,
        maya_router.swapper.address,
        maya_router.swapper.badge.clone(),
        XRD,
        dec!(200),
        swap_memo,
    );

    // Assert
    receipt.expect_commit_success();

    // Perform Send of non-existing asset
    // Act
    let receipt = maya_router.transfer_out(
        maya_router.asgard_vault_1.address,
        maya_router.asgard_vault_1.badge.clone(),
        maya_router.swapper.address,
        *maya_router.resources.get("USDT").unwrap(),
        dec!(100),
        tx_out_memo,
        fee_to_lock,
    );

    // Assert
    receipt.expect_specific_failure(|e| match e {
        RuntimeError::ApplicationError(ApplicationError::PanicMessage(s)) => {
            s.contains("Resource") && s.contains("not available in the vault")
        }
        _ => false,
    });
}

#[test]
fn maya_router_transfer_out_insufficient_balance() {
    // Arrange
    let mut maya_router = MayaRouterSimulator::new();

    // Act
    let tx_out_memo = "OUT:";
    let fee_to_lock = dec!(10);

    // Perform Send from non-existing Asgard Vault
    // Act
    let receipt = maya_router.transfer_out(
        maya_router.asgard_vault_1.address,
        maya_router.asgard_vault_1.badge.clone(),
        maya_router.swapper.address,
        *maya_router.resources.get("USDT").unwrap(),
        dec!(100),
        tx_out_memo,
        fee_to_lock,
    );

    // Assert
    receipt.expect_specific_rejection(|e| match e {
        RejectionReason::ErrorBeforeLoanAndDeferredCostsRepaid(
            RuntimeError::SystemModuleError(SystemModuleError::CostingError(
                CostingError::FeeReserveError(FeeReserveError::InsufficientBalance { .. }),
            )),
        ) => true,
        _ => false,
    });
}

#[test]
fn maya_router_transfer_out_assert_access_rule_failed() {
    // Arrange
    let mut maya_router = MayaRouterSimulator::new();

    // Act
    let swap_memo = "SWAP:MAYA.CACAO";
    let tx_out_memo = "OUT:";
    let fee_to_lock = dec!(10);

    // Perform Swap
    // Act
    let receipt = maya_router.deposit(
        maya_router.asgard_vault_1.address,
        maya_router.swapper.address,
        maya_router.swapper.badge.clone(),
        XRD,
        dec!(100),
        swap_memo,
    );

    // Assert
    receipt.expect_commit_success();

    // Act
    // Expect AssertAccessRuleFailed when using Asgard Vault 2 badge for Asgard Vault 1
    let receipt = maya_router.transfer_out(
        maya_router.asgard_vault_1.address,
        maya_router.asgard_vault_2.badge.clone(),
        maya_router.swapper.address,
        *maya_router.resources.get("USDT").unwrap(),
        dec!(100),
        tx_out_memo,
        fee_to_lock,
    );

    receipt.expect_specific_rejection(|e| {
        matches!(
            e,
            RejectionReason::ErrorBeforeLoanAndDeferredCostsRepaid(RuntimeError::SystemError(
                SystemError::AssertAccessRuleFailed
            ))
        )
    });
}

#[test]
fn maya_router_multiple_asgard_vaults() {
    // Arrange
    let mut maya_router = MayaRouterSimulator::new();
    let swap_memo = "SWAP:MAYA.CACAO";
    let tx_out_memo = "OUT:";

    // Act
    let receipt = maya_router.deposit(
        maya_router.asgard_vault_1.address,
        maya_router.swapper.address,
        maya_router.swapper.badge.clone(),
        XRD,
        dec!(1000),
        swap_memo,
    );
    receipt.expect_commit_success();

    let receipt = maya_router.deposit(
        maya_router.asgard_vault_2.address,
        maya_router.swapper.address,
        maya_router.swapper.badge.clone(),
        XRD,
        dec!(500),
        swap_memo,
    );
    receipt.expect_commit_success();

    let receipt = maya_router.deposit(
        maya_router.asgard_vault_2.address,
        maya_router.swapper.address,
        maya_router.swapper.badge.clone(),
        *maya_router.resources.get("USDT").unwrap(),
        dec!(1000),
        swap_memo,
    );
    receipt.expect_commit_success();

    // No error expected when transferring XRD from Asgard Vault 1 with Asgard Vault 1 badge
    let receipt = maya_router.transfer_out(
        maya_router.asgard_vault_1.address,
        maya_router.asgard_vault_1.badge.clone(),
        maya_router.swapper.address,
        XRD,
        dec!(100),
        tx_out_memo,
        dec!(10),
    );
    receipt.expect_commit_success();

    // No error expected when transferring USDT from Asgard Vault 2 with Asgard Vault 2 badge
    let receipt = maya_router.transfer_out(
        maya_router.asgard_vault_2.address,
        maya_router.asgard_vault_2.badge.clone(),
        maya_router.swapper.address,
        *maya_router.resources.get("USDT").unwrap(),
        dec!(100),
        tx_out_memo,
        dec!(10),
    );
    receipt.expect_commit_success();

    // Expect resources not available when trying to transfer USDT from Asgard Vault 1
    let receipt = maya_router.transfer_out(
        maya_router.asgard_vault_1.address,
        maya_router.asgard_vault_1.badge.clone(),
        maya_router.swapper.address,
        *maya_router.resources.get("USDT").unwrap(),
        dec!(100),
        tx_out_memo,
        dec!(10),
    );
    receipt.expect_specific_failure(|e| match e {
        RuntimeError::ApplicationError(ApplicationError::PanicMessage(s)) => {
            s.contains("Resource") && s.contains("not available in the vault")
        }
        _ => false,
    });
}

#[test]
fn maya_router_move_assets_from_asgard_vault_1_to_asgard_vault_2() {
    // Arrange
    let mut maya_router = MayaRouterSimulator::new();
    let swap_memo = "SWAP:MAYA.CACAO";
    let tx_out_memo = "OUT: AsgardVault transfer";
    let fee_to_lock = dec!(10);

    // Act
    let receipt = maya_router.deposit(
        maya_router.asgard_vault_1.address,
        maya_router.swapper.address,
        maya_router.swapper.badge.clone(),
        XRD,
        dec!(1000),
        swap_memo,
    );
    receipt.expect_commit_success();

    // Transfer XRD from Asgard Vault 1 to Asgard Vault 2
    let receipt = maya_router.transfer_between_vaults(
        maya_router.asgard_vault_1.address,
        maya_router.asgard_vault_1.badge.clone(),
        maya_router.asgard_vault_2.address,
        XRD,
        dec!(990),
        tx_out_memo,
        dec!(10),
    );

    // Assert
    let result = receipt.expect_commit_success();
    let events = result.application_events.as_slice();

    let event_data = events
        .iter()
        .find(|(type_identifier, _)| {
            type_identifier.eq(&EventTypeIdentifier(
                Emitter::Method(maya_router.component_address.into_node_id(), ModuleId::Main),
                "MayaRouterDirectDepositEvent".to_string(),
            ))
        })
        .map(|(_, data)| data)
        .expect("MayaRouterDirectDepositEvent not found");

    assert_eq!(
        scrypto_decode::<MayaRouterDirectDepositEvent>(&event_data).unwrap(),
        MayaRouterDirectDepositEvent {
            resource_address: XRD,
            amount: dec!(990),
            vault_address: maya_router.asgard_vault_2.address,
        }
    );

    // Try Transfer out XRD from Asgard Vault 1
    let receipt = maya_router.transfer_out(
        maya_router.asgard_vault_1.address,
        maya_router.asgard_vault_1.badge.clone(),
        maya_router.swapper.address,
        *maya_router.resources.get("USDT").unwrap(),
        dec!(100),
        tx_out_memo,
        fee_to_lock,
    );

    // Assert
    receipt.expect_specific_rejection(|e| {
        matches!(
            e,
            &RejectionReason::ErrorBeforeLoanAndDeferredCostsRepaid(
                RuntimeError::ApplicationError(ApplicationError::VaultError(
                    VaultError::LockFeeInsufficientBalance { .. }
                ))
            )
        )
    });

    // Transfer out XRD from Asgard Vault 2
    let receipt = maya_router.transfer_out(
        maya_router.asgard_vault_2.address,
        maya_router.asgard_vault_2.badge.clone(),
        maya_router.swapper.address,
        XRD,
        dec!(100),
        tx_out_memo,
        fee_to_lock,
    );
    receipt.expect_commit_success();
}
