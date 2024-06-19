package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"github.com/hyperledger/fabric-protos-go-apiv2/gateway"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
)

const (
	mspID        = "Org1MSP"
	cryptoPath   = "/home/vipr-demo/go/src/github.com/hyperledger/fabric-samples/test-network/organizations/peerOrganizations/org1.example.com"
	certPath     = cryptoPath + "/users/User1@org1.example.com/msp/signcerts/User1@org1.example.com-cert.pem" //for using ca , chnage the /cert.pem
	keyPath      = cryptoPath + "/users/User1@org1.example.com/msp/keystore/"
	tlsCertPath  = cryptoPath + "/peers/peer0.org1.example.com/tls/ca.crt"
	peerEndpoint = "localhost:7051"
	gatewayPeer  = "peer0.org1.example.com"
)

type GitCommit struct {
	CommitHash    string `json:"CommitHash"`
	Repository    string `json:"Repository"`
	CommitMessage string `json:"CommitMessage"`
	Author        string `json:"Author"`
	VersionNumber int    `json:"VersionNumber"`
	Timestamp     string `json:"Timestamp"`
	//RemoteURL     string `json:"RemoteURL"`
}

// PushTransaction struct to match the smart contract definition
type PushTransaction struct {
	Repository string `json:"repository"`
	RemoteURL  string `json:"remoteURL"`
	Timestamp  string `json:"timestamp"`
	Version    int    `json:"version"`
	CommitHash string `json:"commitHash"` // Add this field
}

func main() {

	// Define and parse command-line flags
	var (
		createFlag              bool
		pushFlag                bool
		readFlag                bool
		existsFlag              bool
		getAllFlag              bool
		commitHash              string
		repository              string
		commitMessage           string
		author                  string
		remoteURL               string
		getPushTransactionsFlag bool
		//versionNumber int
	)

	flag.BoolVar(&createFlag, "create", false, "Create a new Git commit")
	flag.BoolVar(&pushFlag, "push", false, "Handle git push")
	flag.BoolVar(&readFlag, "read", false, "Read a Git commit by its hash")
	flag.BoolVar(&existsFlag, "exists", false, "Check if a Git commit exists")
	flag.BoolVar(&getAllFlag, "getAll", false, "Get all Git commits")
	flag.StringVar(&commitHash, "hash", "", "The hash of the Git commit")
	flag.StringVar(&repository, "repo", "", "The repository of the Git commit")
	flag.StringVar(&commitMessage, "message", "", "The commit message")
	flag.StringVar(&author, "author", "", "The author of the Git commit")
	flag.StringVar(&remoteURL, "url", "", "The remote repository URL")
	flag.BoolVar(&getPushTransactionsFlag, "getPushTransactions", false, "Get all push transactions")
	//flag.IntVar(&versionNumber, "version", 0, "The version number of the Git commit")
	// parse flags
	flag.Parse()

	// Setup gRPC connection and client identity
	clientConnection := newGrpcConnection()
	defer clientConnection.Close()

	id := newIdentity()
	sign := newSign()

	gw, err := client.Connect(
		id,
		client.WithSign(sign),
		client.WithClientConnection(clientConnection),
		client.WithEvaluateTimeout(5*time.Second),
		client.WithEndorseTimeout(15*time.Second),
		client.WithSubmitTimeout(5*time.Second),
		client.WithCommitStatusTimeout(1*time.Minute),
	)
	if err != nil {
		fmt.Printf("Failed to connect to gateway: %v\n", err)
		return
	}
	defer gw.Close()

	chaincodeName := "git" // Adjust according to your deployment
	if ccname := os.Getenv("CHAINCODE_NAME"); ccname != "" {
		chaincodeName = ccname
	}
	channelName := "mychannel" // Adjust according to your deployment
	if cname := os.Getenv("CHANNEL_NAME"); cname != "" {
		channelName = cname
	}

	network := gw.GetNetwork(channelName)
	contract := network.GetContract(chaincodeName)

	// Execute smart contract functions based on flags
	if createFlag {
		createGitCommit(contract, commitHash, repository, commitMessage, author)
	} else if readFlag {
		readGitCommit(contract, commitHash)
	} else if pushFlag {
		handleGitPush(contract, repository, remoteURL, commitHash)
	} else if getPushTransactionsFlag {
		getAllPushTransactions(contract)
	} else if existsFlag {
		checkGitCommitExists(contract, commitHash)
	} else if getAllFlag {
		getAllGitCommits(contract)
	} else {
		fmt.Println("No operation specified or unrecognized flag.")
	}
}

func newGrpcConnection() *grpc.ClientConn {
	certificate, err := loadCertificate(tlsCertPath)
	if err != nil {
		panic(err)
	}

	certPool := x509.NewCertPool()
	certPool.AddCert(certificate)
	transportCredentials := credentials.NewClientTLSFromCert(certPool, gatewayPeer)

	connection, err := grpc.Dial(peerEndpoint, grpc.WithTransportCredentials(transportCredentials))
	if err != nil {
		panic(fmt.Errorf("failed to create gRPC connection: %w", err))
	}

	return connection
}

// newIdentity creates a client identity for this Gateway connection using an X.509 certificate.
func newIdentity() *identity.X509Identity {
	certificate, err := loadCertificate(certPath)
	if err != nil {
		panic(err)
	}

	id, err := identity.NewX509Identity(mspID, certificate)
	if err != nil {
		panic(err)
	}

	return id
}

func loadCertificate(filename string) (*x509.Certificate, error) {
	certificatePEM, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}
	return identity.CertificateFromPEM(certificatePEM)
}

// newSign creates a function that generates a digital signature from a message digest using a private key.
func newSign() identity.Sign {
	files, err := os.ReadDir(keyPath)
	if err != nil {
		panic(fmt.Errorf("failed to read private key directory: %w", err))
	}
	privateKeyPEM, err := os.ReadFile(path.Join(keyPath, files[0].Name()))

	if err != nil {
		panic(fmt.Errorf("failed to read private key file: %w", err))
	}

	privateKey, err := identity.PrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		panic(err)
	}

	sign, err := identity.NewPrivateKeySign(privateKey)
	if err != nil {
		panic(err)
	}

	return sign
}

// Implementation of smart contract interaction functions: createGitCommit, readGitCommit, checkGitCommitExists, getAllGitCommits
// These functions will interact with the smart contract based on the flag inputs and perform the respective blockchain transactions
// Omitted for brevity, but would include calling contract.SubmitTransaction() or contract.EvaluateTransaction() with the appropriate function names and arguments from your smart contract
// CreateGitCommit issues a new GitCommit to the world state with given details.
func createGitCommit(contract *client.Contract, commitHash, repository, commitMessage, author string) {
	fmt.Println("--> Submit Transaction: CreateGitCommit")
	_, err := contract.SubmitTransaction("CreateGitCommit", commitHash, repository, commitMessage, author)
	if err != nil {
		fmt.Printf("Failed to submit CreateGitCommit transaction: %v\n", err)
		return
	}
	fmt.Println("CreateGitCommit transaction successfully submitted")
}

// HandleGitPush handles the git push operation, increments the version number, and provides a build message.
func handleGitPush(contract *client.Contract, repository, remoteURL, commitHash string) {
	fmt.Println("--> Submit Transaction: CreateGitPush")
	// Get the latest commit hash
	commitHash, err := getLatestCommitHash()
	if err != nil {
		fmt.Printf("Failed to get latest commit hash: %v\n", err)
		return
	}

	// Append the commit hash to the remote URL
	remoteURLWithHash := fmt.Sprintf("%s", remoteURL)

	fmt.Println("--> Submit Transaction: HandleGitPush")
	result, err := contract.SubmitTransaction("HandleGitPush", repository, remoteURLWithHash, commitHash)
	if err != nil {
		fmt.Printf("Failed to submit HandleGitPush transaction: %v\n", err)
		return
	}
	fmt.Printf("HandleGitPush transaction successfully submitted, result: %s\n", string(result))
}

// Helper function to get the latest commit hash
func getLatestCommitHash() (string, error) {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get latest commit hash: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// gET ALL the push transcation
func getAllPushTransactions(contract *client.Contract) {
	fmt.Println("--> Evaluate Transaction: GetAllPushTransactions")
	result, err := contract.EvaluateTransaction("GetAllPushTransactions")
	if err != nil {
		fmt.Printf("Failed to evaluate GetAllPushTransactions transaction: %v\n", err)
		return
	}
	var pushTransactions []*PushTransaction
	err = json.Unmarshal(result, &pushTransactions)
	if err != nil {
		fmt.Printf("Failed to unmarshal result: %v\n", err)
		return
	}
	// Print the result in a readable format
	prettyResult, err := json.MarshalIndent(pushTransactions, "", "    ")
	if err != nil {
		fmt.Printf("Failed to format result: %v\n", err)
		return
	}
	fmt.Printf("GetAllPushTransactions transaction successfully evaluated, result: %s\n", formatJSON(prettyResult))
}

// ReadGitCommit returns the GitCommit stored in the world state with given commit hash.
func readGitCommit(contract *client.Contract, commitHash string) {
	fmt.Println("--> Evaluate Transaction: ReadGitCommit")
	result, err := contract.EvaluateTransaction("ReadGitCommit", commitHash)
	if err != nil {
		fmt.Printf("Failed to evaluate ReadGitCommit transaction: %v\n", err)
		return
	}
	fmt.Printf("ReadGitCommit transaction successfully evaluated, result: %s\n", string(result))
}

// GitCommitExists checks if a GitCommit with the given commit hash exists in the world state.
func checkGitCommitExists(contract *client.Contract, commitHash string) {
	fmt.Println("--> Evaluate Transaction: GitCommitExists")
	result, err := contract.EvaluateTransaction("GitCommitExists", commitHash)
	if err != nil {
		fmt.Printf("Failed to evaluate GitCommitExists transaction: %v\n", err)
		return
	}
	exists := string(result) == "true"
	fmt.Printf("GitCommitExists transaction successfully evaluated, exists: %v\n", exists)
}

// GetAllGitCommits returns all GitCommits found in the world state.
func getAllGitCommits(contract *client.Contract) {
	fmt.Println("--> Evaluate Transaction: GetAllGitCommits")
	result, err := contract.EvaluateTransaction("GetAllGitCommits")
	if err != nil {
		fmt.Printf("Failed to evaluate GetAllGitCommits transaction: %v\n", err)
		return
	}
	var gitCommits []*GitCommit
	err = json.Unmarshal(result, &gitCommits)
	if err != nil {
		fmt.Printf("Failed to unmarshal result: %v\n", err)
		return
	}
	// Print the result in a readable format
	prettyGitResult, err := json.MarshalIndent(gitCommits, "", "    ")
	if err != nil {
		fmt.Printf("Failed to format result: %v\n", err)
		return
	}
	fmt.Printf("GetAllGitCommits transaction successfully evaluated, result: %s\n", string(prettyGitResult))
}

func exampleErrorHandling(contract *client.Contract) {
	fmt.Println("\n--> Submit Transaction: IncorrectFunction, intentionally failing to demonstrate error handling")

	// Intentionally using an incorrect function name or wrong number of arguments to trigger an error
	_, err := contract.SubmitTransaction("IncorrectFunctionName", "someArgument")
	if err == nil {
		panic("Expected an error but did not receive one.")
	}

	fmt.Println("*** Successfully caught the error:")

	// Handling different types of errors that could be returned
	switch err := err.(type) {
	case *client.EndorseError:
		fmt.Printf("Endorse error for transaction %s with gRPC status %v: %s\n", err.TransactionID, status.Code(err), err)
	case *client.SubmitError:
		fmt.Printf("Submit error for transaction %s with gRPC status %v: %s\n", err.TransactionID, status.Code(err), err)
	case *client.CommitStatusError:
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Printf("Timeout waiting for transaction %s commit status: %s\n", err.TransactionID, err)
		} else {
			fmt.Printf("Error obtaining commit status for transaction %s with gRPC status %v: %s\n", err.TransactionID, status.Code(err), err)
		}
	case *client.CommitError:
		fmt.Printf("Transaction %s failed to commit with status %d: %s\n", err.TransactionID, int32(err.Code), err)
	default:
		panic(fmt.Errorf("unexpected error type %T: %w", err, err))
	}

	// Extracting and displaying error details if available
	statusErr := status.Convert(err)
	if details := statusErr.Details(); len(details) > 0 {
		fmt.Println("Error Details:")
		for _, detail := range details {
			switch detail := detail.(type) {
			case *gateway.ErrorDetail:
				fmt.Printf("- address: %s, mspId: %s, message: %s\n", detail.Address, detail.MspId, detail.Message)
			}
		}
	}
}

func formatJSON(data []byte) string {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, data, "", "    "); err != nil {
		panic(fmt.Errorf("failed to format JSON: %v", err))
	}
	return prettyJSON.String()
}
