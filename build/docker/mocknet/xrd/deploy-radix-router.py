import random
import sys
import time

import requests
from radix_engine_toolkit import (
    Address,
    ManifestBuilder,
    ManifestBuilderAddress,
    Message,
    OwnerRole,
    PrivateKey,
    TransactionBuilder,
    TransactionHeader,
)


class RadixRouterDeployer:
    def __init__(self, base_url, wasm_file_path, rpd_file_path):
        self.base_url = base_url
        self.wasm_file_path = wasm_file_path
        self.rpd_file_path = rpd_file_path
        self.network = {"id": 240, "logical_name": "localnet"}
        # Doesn't matter, just some random key
        self.notary_key = PrivateKey.new_secp256k1(
            bytes.fromhex(
                "76736c78f64e84131218944e6f7ea06d4ec84a2caf363b215a6c951a98bcf41c"
            )
        )

    def run(self):
        package_address = self.publish_package()
        print("Package address " + package_address)
        router_address = self.instantiate_router(package_address)
        print("Router component address " + router_address)
        aggregator_address = self.instantiate_aggregator(package_address)
        print("Aggregator component address " + aggregator_address)

    def publish_package(self):
        wasm_file = open(self.wasm_file_path, mode="rb")
        wasm_bytes = wasm_file.read()
        wasm_file.close()

        rpd_file = open(self.rpd_file_path, mode="rb")
        rpd_bytes = rpd_file.read()
        rpd_file.close()

        receipt = self.submit_manifest_for_success_receipt(
            ManifestBuilder()
            .faucet_lock_fee()
            .package_publish_advanced(OwnerRole.NONE(), wasm_bytes, rpd_bytes, {}, None)
            .build(self.network["id"])
        )
        return receipt["state_updates"]["new_global_entities"][0]["entity_address"]

    def instantiate_router(self, package_address):
        receipt = self.submit_manifest_for_success_receipt(
            ManifestBuilder()
            .faucet_lock_fee()
            .call_function(
                ManifestBuilderAddress.STATIC(Address(package_address)),
                "MayaRouter",
                "instantiate",
                [],
            )
            .build(self.network["id"])
        )
        return receipt["state_updates"]["new_global_entities"][0]["entity_address"]

    def instantiate_aggregator(self, package_address):
        receipt = self.submit_manifest_for_success_receipt(
            ManifestBuilder()
            .faucet_lock_fee()
            .call_function(
                ManifestBuilderAddress.STATIC(Address(package_address)),
                "NoOpAggregator",
                "instantiate",
                [],
            )
            .build(self.network["id"])
        )
        return receipt["state_updates"]["new_global_entities"][0]["entity_address"]

    def submit_manifest_for_success_receipt(self, manifest):
        current_epoch = self.get_current_epoch()
        header = TransactionHeader(
            network_id=self.network["id"],
            start_epoch_inclusive=current_epoch,
            end_epoch_exclusive=current_epoch + 1000,
            nonce=random.randint(0, 0xFFFFFFFF),
            notary_public_key=self.notary_key.public_key(),
            notary_is_signatory=True,
            tip_percentage=0,
        )
        notarized_transaction = (
            TransactionBuilder()
            .header(header)
            .manifest(manifest)
            .message(Message.NONE())
            .notarize_with_private_key(self.notary_key)
        )
        return self.submit_notarized_txn_for_success_receipt(notarized_transaction)

    def submit_notarized_txn_for_success_receipt(self, notarized_transaction):
        intent_hash_string = notarized_transaction.intent_hash().as_str()
        print("Submitting " + intent_hash_string)
        notarized_transaction_hex = "".join(
            hex(i)[2:].zfill(2) for i in notarized_transaction.compile()
        )
        self.post_for_json
        resp = self.post_for_json(
            "/transaction/submit",
            {
                "network": self.network["logical_name"],
                "notarized_transaction_hex": notarized_transaction_hex,
            },
        )
        print(resp)
        max_time_to_wait = time.time() + 10
        while time.time() < max_time_to_wait:
            if self.get_transaction_status(intent_hash_string) == "CommittedSuccess":
                return self.get_transaction_receipt(intent_hash_string)
            time.sleep(1)
        raise Exception(
            "Transaction {intent_hash} did not get committed successfully".format(
                intent_hash=intent_hash_string
            )
        )

    def get_current_epoch(self):
        return self.post_for_json(
            "/status/network-status", {"network": self.network["logical_name"]}
        )["current_epoch_round"]["epoch"]

    def get_transaction_status(self, intent_hash_string):
        return self.post_for_json(
            "/transaction/status",
            {
                "network": self.network["logical_name"],
                "intent_hash": intent_hash_string,
            },
        )["known_payloads"][0]["status"]

    def get_transaction_receipt(self, intent_hash_string):
        return self.post_for_json(
            "/transaction/receipt",
            {
                "network": self.network["logical_name"],
                "intent_hash": intent_hash_string,
            },
        )["committed"]["receipt"]

    def post_for_json(self, relative_path, json):
        resp = requests.post(url=self.base_url + relative_path, json=json)
        if resp.status_code != 200:
            print("An error occurred: " + resp.text)
        resp.raise_for_status()
        return resp.json()


def main():
    deployer = RadixRouterDeployer(sys.argv[1], sys.argv[2], sys.argv[3])
    deployer.run()


if __name__ == "__main__":
    main()
