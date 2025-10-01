package coreapi

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	apiclient "github.com/radixdlt/maya/radix_core_api_client"
	"github.com/radixdlt/maya/radix_core_api_client/models"
	"gitlab.com/mayachain/mayanode/bifrost/pkg/chainclients/radix/types"
)

type CoreApiWrapper struct {
	coreApiClient      *apiclient.RadixCoreApiClient
	network            types.Network
	httpRequestTimeout time.Duration
}

func NewCoreApiWrapper(coreApiClient *apiclient.RadixCoreApiClient, network types.Network, httpRequestTimeout time.Duration) CoreApiWrapper {
	return CoreApiWrapper{
		coreApiClient:      coreApiClient,
		network:            network,
		httpRequestTimeout: httpRequestTimeout,
	}
}

func (w *CoreApiWrapper) GetNetworkStatus() (models.NetworkStatusResponseable, error) {
	ctx, cancel := w.getContextForApiCalls()
	defer cancel()

	statusRequest := models.NewNetworkStatusRequest()
	statusRequest.SetNetwork(&w.network.LogicalName)
	statusResponse, err := w.coreApiClient.Status().NetworkStatus().Post(ctx, statusRequest, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get network status: %w", err)
	}
	return statusResponse, nil
}

func (w *CoreApiWrapper) GetCurrentEpoch() (int64, error) {
	statusResponse, err := w.GetNetworkStatus()
	if err != nil {
		return 0, err
	}
	return *statusResponse.GetCurrentEpochRound().GetEpoch(), nil
}

func (w *CoreApiWrapper) GetCurrentStateVersion() (int64, error) {
	statusResponse, err := w.GetNetworkStatus()
	if err != nil {
		return 0, err
	}
	return *statusResponse.GetCurrentStateIdentifier().GetStateVersion(), nil
}

func (w *CoreApiWrapper) GetSingleTransactionAtStateVersion(stateVersion int64) (*models.CommittedTransactionable, error) {
	ctx, cancel := w.getContextForApiCalls()
	defer cancel()

	req := models.NewStreamTransactionsRequest()
	req.SetFromStateVersion(&stateVersion)
	limit := int32(1)
	req.SetLimit(&limit)
	req.SetNetwork(&w.network.LogicalName)
	sborFormatOptions := models.SborFormatOptions{}
	t := true
	sborFormatOptions.SetProgrammaticJson(&t)
	req.SetSborFormatOptions(&sborFormatOptions)

	txnsResp, err := w.coreApiClient.Stream().Transactions().Post(ctx, req, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get txns: %w", err)
	}

	if *txnsResp.GetCount() == 0 {
		return nil, nil
	}

	txn := txnsResp.GetTransactions()[0]

	return &txn, nil
}

func (w *CoreApiWrapper) SubmitTransaction(notarizedTransactionBytes []byte) error {
	ctx, cancel := w.getContextForApiCalls()
	defer cancel()

	notarizedTransactionHex := hex.EncodeToString(notarizedTransactionBytes)
	req := models.NewTransactionSubmitRequest()
	req.SetNotarizedTransactionHex(&notarizedTransactionHex)
	req.SetNetwork(&w.network.LogicalName)

	_, err := w.coreApiClient.Transaction().Submit().Post(ctx, req, nil)
	if err != nil {
		return fmt.Errorf("failed to submit txn: %w", err)
	}

	return nil
}

func (w *CoreApiWrapper) MethodPreview(componentAddress string, methodName string, args []string) (models.SborDataable, error) {
	ctx, cancel := w.getContextForApiCalls()
	defer cancel()

	req := models.NewTransactionPreviewRequest()

	currentEpoch, err := w.GetCurrentEpoch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current epoch: %w", err)
	}

	manifest := w.buildManifest(componentAddress, args)
	req.SetManifest(&manifest)

	w.setRequestParameters(req, currentEpoch)

	resp, err := w.coreApiClient.Transaction().Preview().Post(ctx, req, nil)
	if err != nil {
		return nil, fmt.Errorf("method call preview failed: %w", err)
	}

	if *resp.GetReceipt().GetStatus() != models.SUCCEEDED_TRANSACTIONSTATUS {
		if resp.GetReceipt().GetErrorMessage() != nil && strings.Contains(*resp.GetReceipt().GetErrorMessage(), "LockFeeInsufficientBalance") {
			return nil, fmt.Errorf("method preview failed with status: %s, the lock fee account is out of funds to simulate the tx fee which is required to get the vault balance", resp.GetReceipt().GetStatus())
		}
		return nil, fmt.Errorf("method preview failed with status: %s", resp.GetReceipt().GetStatus())
	}

	if len(resp.GetReceipt().GetOutput()) == 0 {
		return nil, fmt.Errorf("no instruction outputs in preview response")
	}

	return resp.GetReceipt().GetOutput()[1], nil
}

func (w *CoreApiWrapper) buildManifest(componentAddress string, args []string) string {
	return fmt.Sprintf(`CALL_METHOD 
      Address("account_rdx12y5j239mtr75xh9s2egv3uxtcldvzr8l0wlq4pnh07njs3laqwq7k9")
      "lock_fee" 
      Decimal("10");
  CALL_METHOD 
      Address("%s") 
      "get_vault_balance" 
      Address("%s") 
      Address("%s");`,
		componentAddress,
		args[0],
		args[1],
	)
}

func (w *CoreApiWrapper) setRequestParameters(req *models.TransactionPreviewRequest, currentEpoch int64) {
	startEpochInclusive := currentEpoch
	endEpochExclusive := currentEpoch + 1
	req.SetStartEpochInclusive(&startEpochInclusive)
	req.SetEndEpochExclusive(&endEpochExclusive)

	tipPercentage := int32(0)
	req.SetTipPercentage(&tipPercentage)
	nonce := int64(0)
	req.SetNonce(&nonce)
	req.SetSignerPublicKeys([]models.PublicKeyable{})

	flags := models.NewPreviewFlags()
	useFreeCredit := false
	skipEpochCheck := false
	assumeAllSignatureProofs := true
	flags.SetUseFreeCredit(&useFreeCredit)
	flags.SetSkipEpochCheck(&skipEpochCheck)
	flags.SetAssumeAllSignatureProofs(&assumeAllSignatureProofs)
	req.SetFlags(flags)

	req.SetNetwork(&w.network.LogicalName)
}

func (w *CoreApiWrapper) getContextForApiCalls() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), w.httpRequestTimeout)
}
