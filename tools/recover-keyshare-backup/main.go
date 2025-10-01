package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/itchio/lzma"
	"github.com/rs/zerolog/log"

	"gitlab.com/mayachain/mayanode/bifrost/tss"
	"gitlab.com/mayachain/mayanode/cmd"
	openapi "gitlab.com/mayachain/mayanode/openapi/gen"
	"gitlab.com/mayachain/mayanode/tools/mayascan"
	"gitlab.com/mayachain/mayanode/x/mayachain"
)

////////////////////////////////////////////////////////////////////////////////////////
// Helpers
////////////////////////////////////////////////////////////////////////////////////////

func check(e error, msg string) {
	if e != nil {
		_, file, line, _ := runtime.Caller(1)
		callerLine := fmt.Sprintf("%s:%d", file, line)
		log.Fatal().Msgf("%s: %s\n%s", callerLine, msg, e)
	}
}

func get(url string, result interface{}) error {
	// make the request
	res, err := http.DefaultClient.Get(url)
	if err != nil {
		return err
	}

	// check the status code
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: status code %d", url, res.StatusCode)
	}

	// populate the result
	defer res.Body.Close()
	return json.NewDecoder(res.Body).Decode(result)
}

func selectMember(members []string) (string, error) {
	if len(members) == 0 {
		return "", fmt.Errorf("no options available")
	}

	// display the options
	fmt.Println("Select vault member:")
	for i, option := range members {
		fmt.Printf("%d. %s\n", i+1, option)
	}

	// read user input
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter the number of member: ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("error reading input: %v", err)
	}

	// convert input to integer
	input = strings.TrimSpace(input)
	choice, err := strconv.Atoi(input)
	if err != nil || choice < 1 || choice > len(members) {
		return "", fmt.Errorf("invalid choice, please enter a number between 1 and %d", len(members))
	}

	// return the selected option
	return members[choice-1], nil
}

////////////////////////////////////////////////////////////////////////////////////////
// Main
////////////////////////////////////////////////////////////////////////////////////////

func main() {
	// define command-line flags
	var (
		endpoint = flag.String("endpoint", "", "mayanode endpoint (must contain vault block heights)")
		vault    = flag.String("vault", "", "vault address")
		node     = flag.String("node", "", "node address (member of the vault)")
		mnemonic = flag.String("mnemonic", "", "mnemonic phrase for decryption")
		help     = flag.Bool("help", false, "show help message")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Recover keyshare backup from vault member.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Interactive mode (default)\n")
		fmt.Fprintf(os.Stderr, "  %s\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Non-interactive mode with all flags\n")
		fmt.Fprintf(os.Stderr, "  %s -endpoint https://mayanode.mayachain.info -vault maya1abc... -node maya1xyz... -mnemonic \"word1 word2 ...\"\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Mix of flags and interactive (will prompt for missing values)\n")
		fmt.Fprintf(os.Stderr, "  %s -vault maya1abc... -node maya1xyz...\n", os.Args[0])
	}

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// configure prefixes
	cfg := types.GetConfig()
	cfg.SetBech32PrefixForAccount(cmd.Bech32PrefixAccAddr, cmd.Bech32PrefixAccPub)
	cfg.SetBech32PrefixForValidator(cmd.Bech32PrefixValAddr, cmd.Bech32PrefixValPub)
	cfg.SetBech32PrefixForConsensusNode(cmd.Bech32PrefixConsAddr, cmd.Bech32PrefixConsPub)
	cfg.SetCoinType(cmd.BASEChainCoinType)
	cfg.SetPurpose(cmd.BASEChainCoinPurpose)
	cfg.Seal()

	// handle endpoint
	mayanode := *endpoint
	if mayanode == "" {
		reader := bufio.NewReader(os.Stdin)
		defaultEndpoint := "https://mayanode.mayachain.info"
		fmt.Printf("mayanode (must contain vault block heights) [%s]: ", defaultEndpoint)
		input, err := reader.ReadString('\n')
		check(err, "Failed to read endpoint")
		mayanode = strings.TrimSpace(input)

		if mayanode == "" {
			mayanode = defaultEndpoint
		}
	}

	mayascan.APIEndpoint = mayanode

	// handle vault
	vaultAddr := *vault
	if vaultAddr == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Vault: ")
		input, err := reader.ReadString('\n')
		check(err, "Failed to read vault")
		vaultAddr = strings.TrimSpace(input)
	}

	// get vault response
	vaultResponse := openapi.Vault{}
	vaultUrl := fmt.Sprintf("%s/mayachain/vault/%s", mayanode, vaultAddr)
	err := get(vaultUrl, &vaultResponse)
	check(err, "Failed to get vault")

	// get nodes at vault height
	nodes := []openapi.Node{}
	nodesUrl := fmt.Sprintf("%s/mayachain/nodes?height=%d", mayanode, *vaultResponse.StatusSince)
	err = get(nodesUrl, &nodes)
	check(err, "Failed to get nodes")

	// filter node addresses that are members
	memberAddresses := []string{}
	for _, n := range nodes {
		for _, member := range n.SignerMembership {
			if member == vaultAddr {
				memberAddresses = append(memberAddresses, n.NodeAddress)
				break
			}
		}
	}

	// check if we found any members
	if len(memberAddresses) == 0 {
		log.Fatal().Msg("No members found for the specified vault. The vault might be inactive or the node addresses could not be retrieved.")
	}

	// create a map for O(1) membership checks
	memberMap := make(map[string]struct{})
	for _, member := range memberAddresses {
		memberMap[member] = struct{}{}
	}

	// handle node selection
	selectedNode := *node
	if selectedNode == "" {
		// interactive mode - let user select
		selectedNode, err = selectMember(memberAddresses)
		check(err, "Failed to select node")
	} else {
		// validate the provided node is a member using O(1) lookup
		if _, found := memberMap[selectedNode]; !found {
			log.Fatal().Msgf("Node %s is not a member of vault %s", selectedNode, vaultAddr)
		}
	}

	// scan from 20 blocks before vault block height to find corresponding TssPool
	var keyshare []byte
	var keyshareEddsa []byte
	start := *vaultResponse.BlockHeight - 20
	stop := *vaultResponse.BlockHeight
	for block := range mayascan.Scan(int(start), int(stop)) {
		for _, tx := range block.Txs {
			for _, msg := range tx.Tx.GetMsgs() {
				msgTssPool, ok := msg.(*mayachain.MsgTssPool)
				if !ok {
					continue
				}
				if msgTssPool.Signer.String() != selectedNode {
					continue
				}

				// error if keyshare is missing
				if msgTssPool.KeysharesBackup == nil {
					log.Fatal().Msg("Keyshare was not backed up.")
				}
				keyshare = msgTssPool.KeysharesBackup
				keyshareEddsa = msgTssPool.KeysharesBackupEddsa
				break
			}
		}
		if keyshare != nil {
			break
		}
	}

	// handle mnemonic
	mnemonicPhrase := *mnemonic
	if mnemonicPhrase == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Mnemonic: ")
		var input string
		input, err = reader.ReadString('\n')
		check(err, "Failed to read mnemonic")
		mnemonicPhrase = strings.TrimSpace(input)
	}

	// decrypt keyshare
	decrypted, err := tss.DecryptKeyshares(keyshare, mnemonicPhrase)
	check(err, "Failed to decrypt keyshare")

	// decompress lzma
	cmpDec := lzma.NewReader(bytes.NewReader(decrypted))

	// write to file
	filename := fmt.Sprintf("%s-localstate-%s.json", selectedNode, vaultAddr)
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	check(err, "Failed to open file")
	defer f.Close()
	_, err = io.Copy(f, cmpDec)
	check(err, "Failed to write to file")

	// success
	fmt.Printf("Decrypted keyshare written to %s\n", filename)

	if len(keyshareEddsa) > 0 {
		// decrypt eddsa keyshare
		decryptedEddsa, err := tss.DecryptKeyshares(keyshareEddsa, mnemonicPhrase)
		check(err, "Failed to decrypt keyshare")

		cmpDecEddsa := lzma.NewReader(bytes.NewReader(decryptedEddsa))

		filenameEddsa := fmt.Sprintf("localstate-%s.json", *vaultResponse.PubKeyEddsa)
		fed, err := os.OpenFile(filenameEddsa, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		check(err, "Failed to open file")
		defer fed.Close()
		_, err = io.Copy(fed, cmpDecEddsa)
		check(err, "Failed to write to file")

		// success
		fmt.Printf("Decrypted eddsa keyshare written to %s\n", filenameEddsa)
	}
}
