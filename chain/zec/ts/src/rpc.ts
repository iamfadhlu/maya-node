import axios from "axios";

import { Config, UTXO } from "./types";

export class ZcashRPCClient {
  private config: Config;

  constructor(config: Config) {
    this.config = config;
  }

  private async request(method: string, params: any[] = []): Promise<any> {
    const url = `http://${this.config.host}:${this.config.port}`;

    try {
      const response = await axios.post(
        url,
        {
          jsonrpc: "2.0",
          method,
          params,
          id: 1,
        },
        {
          headers: { "Content-Type": "application/json" },
          auth: {
            username: this.config.username,
            password: this.config.password,
          },
          timeout: 10000,
        },
      );

      if (response.data.error) {
        throw new Error(`RPC Error: ${response.data.error.message}`);
      }

      return response.data.result;
    } catch (error) {
      if (axios.isAxiosError(error)) {
        throw new Error(`RPC request failed: ${error.message}`);
      }
      throw error;
    }
  }

  async getNetworkInfo(): Promise<any> {
    return this.request("getnetworkinfo");
  }

  async getBlockchainInfo(): Promise<any> {
    return this.request("getblockchaininfo");
  }

  async validateAddress(address: string): Promise<any> {
    return this.request("validateaddress", [address]);
  }

  async getBalance(address: string): Promise<number> {
    const utxos = await this.getUTXOs(address);
    return utxos.reduce((sum, utxo) => sum + utxo.value, 0) / 100000000; // Convert zatoshis to ZEC
  }

  async getUTXOs(address: string): Promise<UTXO[]> {
    try {
      const rawUtxos = await this.request("getaddressutxos", [address]);

      return rawUtxos.map((utxo: any) => ({
        txid: utxo.txid,
        vout: utxo.outputIndex,
        value: utxo.satoshis,
        height: utxo.height || 0,
      }));
    } catch (error) {
      console.error("Failed to get UTXOs:", error);
      return [];
    }
  }

  async sendRawTransaction(hexTx: string): Promise<string> {
    return this.request("sendrawtransaction", [hexTx]);
  }

  async getRawTransaction(
    txid: string,
    verbose: boolean = false,
  ): Promise<any> {
    return this.request("getrawtransaction", [txid, verbose]);
  }
}
