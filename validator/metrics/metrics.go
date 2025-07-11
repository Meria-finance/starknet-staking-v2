package metrics

import (
	"context"
	"net/http"
	"time"

	"github.com/NethermindEth/juno/utils"
	"github.com/NethermindEth/starknet-staking-v2/validator/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var _ Tracer = (*Metrics)(nil)

// Metrics represents the metrics server for the validator
type Metrics struct {
	server                          *http.Server
	logger                          *utils.ZapLogger
	network                         string
	registry                        *prometheus.Registry
	latestBlockNumber               *prometheus.GaugeVec
	currentEpochID                  *prometheus.GaugeVec
	currentEpochLength              *prometheus.GaugeVec
	currentEpochStartingBlockNumber *prometheus.GaugeVec
	currentEpochAssignedBlockNumber *prometheus.GaugeVec
	lastAttestationTimestamp        *prometheus.GaugeVec
	attestationSubmittedCount       *prometheus.CounterVec
	attestationFailureCount         *prometheus.CounterVec
	attestationConfirmedCount       *prometheus.CounterVec
	signerBalance                   *prometheus.GaugeVec
	signerBalanceBelowThreshold     *prometheus.GaugeVec
}

// NewMetrics creates a new metrics server
func NewMetrics(serverAddress string, chainID string, logger *utils.ZapLogger) *Metrics {
	registry := prometheus.NewRegistry()

	m := &Metrics{
		logger:   logger,
		network:  chainID,
		registry: registry,
		latestBlockNumber: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "validator_attestation_starknet_latest_block_number",
				Help: "The latest block number seen by the validator on the Starknet network",
			},
			[]string{"network"},
		),
		currentEpochID: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "validator_attestation_current_epoch_id",
				Help: "The ID of the current epoch the validator is participating in",
			},
			[]string{"network"},
		),
		currentEpochLength: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "validator_attestation_current_epoch_length",
				Help: "The total length (in blocks) of the current epoch",
			},
			[]string{"network"},
		),
		currentEpochStartingBlockNumber: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "validator_attestation_current_epoch_starting_block_number",
				Help: "The first block number of the current epoch",
			},
			[]string{"network"},
		),
		currentEpochAssignedBlockNumber: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "validator_attestation_current_epoch_assigned_block_number",
				Help: "The specific block number within the current epoch for which the validator is assigned to attest",
			},
			[]string{"network"},
		),
		lastAttestationTimestamp: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "validator_attestation_last_attestation_timestamp_seconds",
				Help: "The Unix timestamp (in seconds) of the last successful attestation submission",
			},
			[]string{"network"},
		),
		attestationSubmittedCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "validator_attestation_attestation_submitted_count",
				Help: "The total number of attestations submitted by the validator since startup",
			},
			[]string{"network"},
		),
		attestationFailureCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "validator_attestation_attestation_failure_count",
				Help: "The total number of attestation transaction submission failures encountered by the validator since startup",
			},
			[]string{"network"},
		),
		attestationConfirmedCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "validator_attestation_attestation_confirmed_count",
				Help: "The total number of attestations that have been confirmed on the network since validator startup",
			},
			[]string{"network"},
		),
		signerBalance: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "validator_attestation_signer_balance",
				Help: "The balance of the account that signs the attestation after each attest transaction",
			},
			[]string{"network"},
		),
		signerBalanceBelowThreshold: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "validator_attestation_signer_below_threshold",
				Help: "Set to one if the account that signs the attestation has it's balance below certain threshold",
			},
			[]string{"network"},
		),
	}

	// Register metrics with Prometheus registry
	registry.MustRegister(
		m.latestBlockNumber,
		m.currentEpochID,
		m.currentEpochLength,
		m.currentEpochStartingBlockNumber,
		m.currentEpochAssignedBlockNumber,
		m.lastAttestationTimestamp,
		m.attestationSubmittedCount,
		m.attestationFailureCount,
		m.attestationConfirmedCount,
		m.signerBalance,
		m.signerBalanceBelowThreshold,
	)

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		if err != nil {
			m.logger.Errorf("Failed to write health check response: %v", err)
		}
	})
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	m.server = &http.Server{
		Addr:    serverAddress,
		Handler: mux,
	}

	return m
}

// Start starts the metrics server
func (m *Metrics) Start() error {
	m.logger.Infof("Starting metrics server on %s", m.server.Addr)
	return m.server.ListenAndServe()
}

// Stop stops the metrics server
func (m *Metrics) Stop(ctx context.Context) error {
	m.logger.Info("Stopping metrics server")
	return m.server.Shutdown(ctx)
}

// UpdateLatestBlockNumber updates the latest block number metric
func (m *Metrics) UpdateLatestBlockNumber(blockNumber uint64) {
	m.logger.Debugw("UpdateLatestBlockNumber", "blockNumber", blockNumber)
	m.latestBlockNumber.WithLabelValues(m.network).Set(float64(blockNumber))
}

// UpdateEpochInfo updates the epoch-related metrics
func (m *Metrics) UpdateEpochInfo(epochInfo *types.EpochInfo, targetBlock uint64) {
	m.logger.Debugw("UpdateEpochInfo", "epochInfo", epochInfo, "targetBlock", targetBlock)
	m.currentEpochID.WithLabelValues(m.network).Set(float64(epochInfo.EpochId))
	m.currentEpochLength.WithLabelValues(m.network).Set(float64(epochInfo.EpochLen))
	m.currentEpochStartingBlockNumber.
		WithLabelValues(m.network).
		Set(float64(epochInfo.StartingBlock.Uint64()))
	m.currentEpochAssignedBlockNumber.WithLabelValues(m.network).Set(float64(targetBlock))
}

// UpdateSignerBalance set's the signer account balance. If it is too big a default max value is set
// instead
func (m *Metrics) UpdateSignerBalance(balance float64) {
	m.logger.Debugw("UpdateSignerBalancer", "balance", balance)
	m.signerBalance.WithLabelValues(m.network).Set(balance)
}

// RecordAttestationSubmitted increments the attestation submitted counter
func (m *Metrics) RecordAttestationSubmitted() {
	m.logger.Debugw("RecordAttestationSubmitted")
	m.attestationSubmittedCount.WithLabelValues(m.network).Inc()
	m.lastAttestationTimestamp.WithLabelValues(m.network).Set(float64(time.Now().Unix()))
}

// RecordAttestationFailure increments the attestation failure counter
func (m *Metrics) RecordAttestationFailure() {
	m.logger.Debugw("RecordAttestationFailure")
	m.attestationFailureCount.WithLabelValues(m.network).Inc()
}

// RecordAttestationConfirmed increments the attestation confirmed counter
func (m *Metrics) RecordAttestationConfirmed() {
	m.logger.Debugw("RecordAttestationConfirmed")
	m.attestationConfirmedCount.WithLabelValues(m.network).Inc()
}

// RecordSignerBalanceAboveThreshold sets the value to 0
func (m *Metrics) RecordSignerBalanceAboveThreshold() {
	m.logger.Debug("RecordSignerBalanceAboveThreshold")
	m.signerBalanceBelowThreshold.WithLabelValues(m.network).Set(0)
}

// RecordSignerBalanceBelowThreshold sets the value to 1
func (m *Metrics) RecordSignerBalanceBelowThreshold() {
	m.logger.Debug("RecordSignerBalanceBelowThreshold")
	m.signerBalanceBelowThreshold.WithLabelValues(m.network).Set(1)
}
