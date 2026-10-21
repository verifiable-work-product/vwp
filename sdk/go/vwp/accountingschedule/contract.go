// Package accountingschedule defines the executable, versioned accounting
// schedule artifact shared by VWP authors, approval services and the ledger.
package accountingschedule

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	ArtifactPath     = "calculations/accounting-schedule.json"
	SchemaVersion    = 1
	MaxEntries       = 1000
	MaxArtifactBytes = 2 << 20
)

type Artifact struct {
	SchemaVersion int     `json:"schemaVersion"`
	Name          string  `json:"name"`
	Kind          string  `json:"kind"`
	Currency      string  `json:"currency"`
	AmountScale   int     `json:"amountScale"`
	StartDate     string  `json:"startDate"`
	EndDate       string  `json:"endDate"`
	TotalAmount   string  `json:"totalAmount"`
	Entries       []Entry `json:"entries"`
}

type Entry struct {
	ID           string    `json:"id"`
	Date         string    `json:"date"`
	Description  string    `json:"description"`
	ObligationID string    `json:"obligationId,omitempty"`
	Postings     []Posting `json:"postings"`
}

type Posting struct {
	AccountRole string            `json:"accountRole"`
	Debit       string            `json:"debit,omitempty"`
	Credit      string            `json:"credit,omitempty"`
	Dimensions  map[string]string `json:"dimensions,omitempty"`
}

// CreationConfig is selected by the reviewer. A schedule always targets its
// source workspace; every role is bound to an existing ledger account ID.
type CreationConfig struct {
	JournalID       string            `json:"journalId"`
	SubsidiaryID    string            `json:"subsidiaryId,omitempty"`
	AccountBindings map[string]string `json:"accountBindings"`
}

type SourceReference struct {
	OrgID          string `json:"orgId"`
	WorkspaceID    string `json:"workspaceId"`
	WorkproductID  string `json:"workproductId"`
	VersionHash    string `json:"versionHash"`
	ArtifactPath   string `json:"artifactPath"`
	ArtifactSHA256 string `json:"artifactSha256"`
}

type Approval struct {
	ID            string    `json:"id"`
	PrincipalID   string    `json:"principalId"`
	Role          string    `json:"role"`
	ApprovedAt    time.Time `json:"approvedAt"`
	VersionHash   string    `json:"versionHash"`
	ContentDigest string    `json:"contentDigest"`
}

// ApprovedSource is returned only after the owning VWP service verifies the
// approval policy against the immutable package version and recorded intent.
type ApprovedSource struct {
	CreationID         string          `json:"creationId"`
	Source             SourceReference `json:"source"`
	Artifact           Artifact        `json:"artifact"`
	ArtifactBytes      []byte          `json:"artifactBytes"`
	Config             CreationConfig  `json:"config"`
	Approvals          []Approval      `json:"approvals"`
	ContentDigest      string          `json:"contentDigest"`
	AttestationsDigest string          `json:"attestationsDigest"`
	PolicyHash         string          `json:"policyHash"`
	ApprovedAt         time.Time       `json:"approvedAt"`
}

type CreateRequest struct {
	CreationID string          `json:"creationId"`
	Source     SourceReference `json:"source"`
	Config     CreationConfig  `json:"config"`
}

// Schedule kinds. Each names the accounting event a schedule's entries
// book; the extension validator for the producing template fixes the
// account-role vocabulary and ties every entry to its calculation.
const (
	KindRevenueRecognition     = "revenue_recognition"
	KindDepreciation           = "depreciation"
	KindFairValueRemeasurement = "fair_value_remeasurement"
	KindImpairment             = "impairment"
	KindStakingRewards         = "staking_rewards"
	KindInterestAccrual        = "interest_accrual"
	KindRealizedGainLoss       = "realized_gain_loss"
)

var kinds = map[string]bool{
	KindRevenueRecognition: true, KindDepreciation: true, KindFairValueRemeasurement: true,
	KindImpairment: true, KindStakingRewards: true, KindInterestAccrual: true, KindRealizedGainLoss: true,
}

// Kinds lists the accepted schedule kinds in a stable order.
func Kinds() []string {
	out := make([]string, 0, len(kinds))
	for k := range kinds {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var decimalPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,23})(\.[0-9]+)?$`)
var rolePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// Parse rejects unknown fields, duplicate JSON keys, omitted precision and
// malformed decimal strings. Hash the input bytes, not a re-encoded value.
func Parse(raw []byte) (*Artifact, error) {
	if len(raw) == 0 || len(raw) > MaxArtifactBytes {
		return nil, fmt.Errorf("accounting schedule must be between 1 and %d bytes", MaxArtifactBytes)
	}
	if err := uniqueKeys(json.NewDecoder(bytes.NewReader(raw))); err != nil {
		return nil, fmt.Errorf("accounting schedule JSON: %w", err)
	}
	if err := validateJSONShape(raw); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("accounting schedule JSON: %w", err)
	}
	if v, ok := fields["amountScale"]; !ok || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		return nil, fmt.Errorf("amountScale is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var a Artifact
	if err := decoder.Decode(&a); err != nil {
		return nil, fmt.Errorf("accounting schedule JSON: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("unexpected trailing JSON")
	}
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return &a, nil
}

func uniqueKeys(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	if delim, ok := token.(json.Delim); ok {
		switch delim {
		case '{':
			keys := map[string]bool{}
			for d.More() {
				key, err := d.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok {
					return fmt.Errorf("invalid object key")
				}
				if keys[name] {
					return fmt.Errorf("duplicate key %q", name)
				}
				keys[name] = true
				if err := uniqueKeys(d); err != nil {
					return err
				}
			}
		case '[':
			for d.More() {
				if err := uniqueKeys(d); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("unexpected delimiter")
		}
		_, err = d.Token()
	}
	return err
}

// Decimal returns an exact positive-or-zero amount at the declared precision.
// It deliberately rejects exponents, signed values, NaN, fractions and floats.
func Decimal(value string, scale int) (*big.Rat, error) {
	if scale < 0 || scale > 8 || !decimalPattern.MatchString(value) {
		return nil, fmt.Errorf("invalid decimal amount %q", value)
	}
	if dot := strings.IndexByte(value, '.'); dot >= 0 && len(value)-dot-1 > scale {
		return nil, fmt.Errorf("amount %q exceeds precision %d", value, scale)
	}
	r, ok := new(big.Rat).SetString(value)
	if !ok {
		return nil, fmt.Errorf("invalid decimal amount %q", value)
	}
	return r, nil
}

func (a Artifact) Validate() error {
	if a.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported accounting schedule schemaVersion %d", a.SchemaVersion)
	}
	if strings.TrimSpace(a.Name) == "" || len(a.Name) > 240 {
		return fmt.Errorf("name must contain 1 to 240 characters")
	}
	if !kinds[a.Kind] {
		return fmt.Errorf("unsupported accounting schedule kind %q", a.Kind)
	}
	if !currencyPattern.MatchString(a.Currency) {
		return fmt.Errorf("currency must be an uppercase three-letter code")
	}
	if a.AmountScale < 0 || a.AmountScale > 8 {
		return fmt.Errorf("amountScale must be between 0 and 8")
	}
	start, err := time.Parse(time.DateOnly, a.StartDate)
	if err != nil || start.Year() < 1 {
		return fmt.Errorf("startDate must be YYYY-MM-DD")
	}
	end, err := time.Parse(time.DateOnly, a.EndDate)
	if err != nil || end.Before(start) {
		return fmt.Errorf("endDate must be YYYY-MM-DD and on or after startDate")
	}
	total, err := Decimal(a.TotalAmount, a.AmountScale)
	if err != nil || total.Sign() <= 0 {
		return fmt.Errorf("totalAmount must be a positive decimal at amountScale")
	}
	if len(a.Entries) == 0 || len(a.Entries) > MaxEntries {
		return fmt.Errorf("entries must contain between 1 and %d entries", MaxEntries)
	}
	ids := map[string]bool{}
	sum := new(big.Rat)
	for _, entry := range a.Entries {
		if strings.TrimSpace(entry.ID) == "" || len(entry.ID) > 120 || ids[entry.ID] {
			return fmt.Errorf("entry ID %q must be nonempty, unique and at most 120 characters", entry.ID)
		}
		ids[entry.ID] = true
		date, err := time.Parse(time.DateOnly, entry.Date)
		if err != nil || date.Before(start) || date.After(end) {
			return fmt.Errorf("entry %s date must fall within the schedule dates", entry.ID)
		}
		if strings.TrimSpace(entry.Description) == "" || len(entry.Description) > 1000 {
			return fmt.Errorf("entry %s description must contain 1 to 1000 characters", entry.ID)
		}
		if len(entry.Postings) < 2 || len(entry.Postings) > 100 {
			return fmt.Errorf("entry %s must contain 2 to 100 postings", entry.ID)
		}
		debits, credits := new(big.Rat), new(big.Rat)
		for _, posting := range entry.Postings {
			if !rolePattern.MatchString(posting.AccountRole) {
				return fmt.Errorf("entry %s has invalid accountRole %q", entry.ID, posting.AccountRole)
			}
			if (posting.Debit == "") == (posting.Credit == "") {
				return fmt.Errorf("entry %s posting requires exactly one debit or credit", entry.ID)
			}
			value, side := posting.Debit, debits
			if posting.Credit != "" {
				value, side = posting.Credit, credits
			}
			amount, err := Decimal(value, a.AmountScale)
			if err != nil || amount.Sign() <= 0 {
				return fmt.Errorf("entry %s postings require positive decimals at amountScale", entry.ID)
			}
			side.Add(side, amount)
			if len(posting.Dimensions) > 20 {
				return fmt.Errorf("entry %s has too many dimensions", entry.ID)
			}
			for key, value := range posting.Dimensions {
				if strings.TrimSpace(key) == "" || len(key) > 100 || strings.TrimSpace(value) == "" || len(value) > 240 {
					return fmt.Errorf("entry %s has invalid dimensions", entry.ID)
				}
			}
		}
		if debits.Cmp(credits) != 0 {
			return fmt.Errorf("entry %s does not balance", entry.ID)
		}
		sum.Add(sum, debits)
	}
	if sum.Cmp(total) != 0 {
		return fmt.Errorf("totalAmount does not equal the sum of entry debits")
	}
	return nil
}

func (a Artifact) RequiredAccountRoles() []string {
	roles := map[string]bool{}
	for _, entry := range a.Entries {
		for _, posting := range entry.Postings {
			roles[posting.AccountRole] = true
		}
	}
	out := make([]string, 0, len(roles))
	for role := range roles {
		out = append(out, role)
	}
	sort.Strings(out)
	return out
}

func (c CreationConfig) Validate(a Artifact) error {
	if strings.TrimSpace(c.JournalID) == "" {
		return fmt.Errorf("journalId is required")
	}
	roles := a.RequiredAccountRoles()
	if len(c.AccountBindings) != len(roles) {
		return fmt.Errorf("accountBindings must map exactly the required account roles")
	}
	for _, role := range roles {
		if strings.TrimSpace(c.AccountBindings[role]) == "" {
			return fmt.Errorf("account binding required for %s", role)
		}
	}
	return nil
}

func Hash(raw []byte) string {
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

// CreationID is stable across lost acknowledgements, client retries and worker
// restarts. Changing reviewer-selected config cannot mint a second schedule.
func CreationID(source SourceReference) string {
	raw, _ := json.Marshal([]string{source.OrgID, source.WorkspaceID, source.WorkproductID, source.VersionHash, source.ArtifactPath})
	return "acs_" + Hash(raw)
}

// DecodeStrict is useful for service contracts as well as artifact JSON.
func DecodeStrict(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("unexpected trailing JSON")
	}
	return nil
}
