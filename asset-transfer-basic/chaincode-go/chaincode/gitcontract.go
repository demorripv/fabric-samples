package chaincode

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// SmartContract provides functions for managing GitCommits
type SmartContract struct {
	contractapi.Contract
}

// GitCommit describes basic details of what makes up a Git commit
type GitCommit struct {
	CommitHash    string `json:"CommitHash"`
	Repository    string `json:"Repository"`
	CommitMessage string `json:"CommitMessage"`
	Author        string `json:"Author"`
	VersionNumber int    `json:"VersionNumber"`
	Timestamp     string `json:"Timestamp"`
	//RemoteURL     string `json:"RemoteURL"`
}

type PushTransaction struct {
	Repository string `json:"repository"`
	RemoteURL  string `json:"remoteURL"`
	Timestamp  string `json:"timestamp"`
	Version    int    `json:"version"`
	CommitHash string `json:"CommitHash"`
}

// RepositoryVersion tracks the current version number of a repository
type RepositoryVersion struct {
	Repository    string `json:"Repository"`
	VersionNumber int    `json:"VersionNumber"`
}

// InitLedger adds a base set of GitCommits to the ledger
func (s *SmartContract) InitLedger(ctx contractapi.TransactionContextInterface) error {
	gitCommits := []GitCommit{
		// Sample GitCommits - you can modify this with real data
		{CommitHash: "hash1", Repository: "repo1", CommitMessage: "Initial commit", Author: "Alice", VersionNumber: 1, Timestamp: "2023-06-01T12:00:00Z"},
		{CommitHash: "hash2", Repository: "repo2", CommitMessage: "Added feature", Author: "Bob", VersionNumber: 1, Timestamp: "2023-06-02T12:00:00Z"},
		// Add more GitCommits if necessary
	}

	for _, gitCommit := range gitCommits {
		gitCommitJSON, err := json.Marshal(gitCommit)
		if err != nil {
			return err
		}

		err = ctx.GetStub().PutState(gitCommit.CommitHash, gitCommitJSON)
		if err != nil {
			return fmt.Errorf("failed to put to world state. %v", err)
		}
	}

	// Initialize repository versions
	repoVersions := []RepositoryVersion{
		{Repository: "repo1", VersionNumber: 1},
		{Repository: "repo2", VersionNumber: 1},
		{Repository: "git-test", VersionNumber: 1},
	}

	for _, repoVersion := range repoVersions {
		repoVersionJSON, err := json.Marshal(repoVersion)
		if err != nil {
			return err
		}

		err = ctx.GetStub().PutState("VERSION_"+repoVersion.Repository, repoVersionJSON)
		if err != nil {
			return fmt.Errorf("failed to put to world state: %v", err)
		}
	}
	return nil
}

// CreateGitCommit issues a new GitCommit to the world state with given details.
func (s *SmartContract) CreateGitCommit(ctx contractapi.TransactionContextInterface, commitHash string, repository string, commitMessage string, author string) error {
	exists, err := s.GitCommitExists(ctx, commitHash)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("the commit %s already exists", commitHash)
	}

	repoVersion, err := s.GetRepositoryVersion(ctx, repository)
	if err != nil {
		if err.Error() == fmt.Sprintf("the repository %s does not have a version number", repository) {
			repoVersion = &RepositoryVersion{Repository: repository, VersionNumber: 1}
			err = s.SetRepositoryVersion(ctx, repoVersion)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	gitCommit := GitCommit{
		CommitHash:    commitHash,
		Repository:    repository,
		CommitMessage: commitMessage,
		Author:        author,
		VersionNumber: repoVersion.VersionNumber,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}
	gitCommitJSON, err := json.Marshal(gitCommit)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState(commitHash, gitCommitJSON)
}

// ReadGitCommit returns the GitCommit stored in the world state with given commit hash.
func (s *SmartContract) ReadGitCommit(ctx contractapi.TransactionContextInterface, commitHash string) (*GitCommit, error) {
	gitCommitJSON, err := ctx.GetStub().GetState(commitHash)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %v", err)
	}
	if gitCommitJSON == nil {
		return nil, fmt.Errorf("the commit %s does not exist", commitHash)
	}

	var gitCommit GitCommit
	err = json.Unmarshal(gitCommitJSON, &gitCommit)
	if err != nil {
		return nil, err
	}

	return &gitCommit, nil
}

// GitCommitExists returns true when a GitCommit with the given commit hash exists in the world state.
func (s *SmartContract) GitCommitExists(ctx contractapi.TransactionContextInterface, commitHash string) (bool, error) {
	gitCommitJSON, err := ctx.GetStub().GetState(commitHash)
	if err != nil {
		return false, fmt.Errorf("failed to read from world state: %v", err)
	}

	return gitCommitJSON != nil, nil
}

// GetAllGitCommits returns all GitCommits found in the world state.
func (s *SmartContract) GetAllGitCommits(ctx contractapi.TransactionContextInterface) ([]*GitCommit, error) {
	resultsIterator, err := ctx.GetStub().GetStateByRange("", "")
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var gitCommits []*GitCommit
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		var gitCommit GitCommit
		err = json.Unmarshal(queryResponse.Value, &gitCommit)
		if err != nil {
			return nil, err
		}
		gitCommits = append(gitCommits, &gitCommit)
	}

	// Sort gitCommits by timestamp
	sort.Slice(gitCommits, func(i, j int) bool {
		return gitCommits[i].Timestamp < gitCommits[j].Timestamp
	})
	// Format the output in a readable JSON format
	prettyGIt, err := json.MarshalIndent(gitCommits, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to format result: %v", err)
	}

	fmt.Printf("GetAllGitCommits transaction successfully evaluated, result:\n%s\n", string(prettyGIt))
	return gitCommits, nil
}

// IncrementVersionNumber increments the version number of a repository.
func (s *SmartContract) IncrementVersionNumber(ctx contractapi.TransactionContextInterface, repository string) error {
	repoVersion, err := s.GetRepositoryVersion(ctx, repository)
	if err != nil {
		return err
	}

	repoVersion.VersionNumber++
	return s.SetRepositoryVersion(ctx, repoVersion)

}

// GetRepositoryVersion retrieves the current version number for a repository.
func (s *SmartContract) GetRepositoryVersion(ctx contractapi.TransactionContextInterface, repository string) (*RepositoryVersion, error) {
	repoVersionJSON, err := ctx.GetStub().GetState("VERSION_" + repository)
	if err != nil {
		return nil, fmt.Errorf("failed to read from world state: %v", err)
	}
	if repoVersionJSON == nil {
		return nil, fmt.Errorf("the repository %s does not have a version number", repository)
	}

	var repoVersion RepositoryVersion
	err = json.Unmarshal(repoVersionJSON, &repoVersion)
	if err != nil {
		return nil, err
	}

	return &repoVersion, nil
}

// SetRepositoryVersion sets the current version number for a repository.
func (s *SmartContract) SetRepositoryVersion(ctx contractapi.TransactionContextInterface, repoVersion *RepositoryVersion) error {
	repoVersionJSON, err := json.Marshal(repoVersion)
	if err != nil {
		return err
	}

	return ctx.GetStub().PutState("VERSION_"+repoVersion.Repository, repoVersionJSON)
}

// HandleGitPush handles the git push operation, increments the version number, and provides a build message.
// HandleGitPush handles the git push operation, increments the version number, and provides a build message.
func (s *SmartContract) HandleGitPush(ctx contractapi.TransactionContextInterface, repository, remoteURL, commitHash string) (string, error) {
	err := s.IncrementVersionNumber(ctx, repository)
	if err != nil {
		return "", err
	}

	// Fetch the commit using the provided commit hash ADDED NEW
	commitJSON, err := ctx.GetStub().GetState(commitHash)
	if err != nil {
		return "", fmt.Errorf("failed to get commit: %s", err.Error())
	}
	if commitJSON == nil {
		return "", fmt.Errorf("commit not found: %s", commitHash)
	}

	var lastCommit GitCommit
	err = json.Unmarshal(commitJSON, &lastCommit)
	if err != nil {
		return "", err
	}

	if lastCommit.Repository != repository {
		return "", fmt.Errorf("commit repository mismatch: expected %s, got %s", repository, lastCommit.Repository)
	}

	// Update the last commit with the remote URL
	remoteURLWithHash := fmt.Sprintf("%s", remoteURL)

	//lastCommit.RemoteURL = remoteURLWithHash

	// Save the updated commit
	updatedCommitJSON, err := json.Marshal(lastCommit)
	if err != nil {
		return "", err
	}

	err = ctx.GetStub().PutState(commitHash, updatedCommitJSON)
	if err != nil {
		return "", err
	}

	// Store the remote URL in the latest commit OLD
	//resultsIterator, err := ctx.GetStub().GetStateByRange("", "")
	//if err != nil {
	//	return "", err
	//}
	//defer resultsIterator.Close()

	//var lastCommit *GitCommit
	//for resultsIterator.HasNext() {
	//	queryResponse, err := resultsIterator.Next()
	//	if err != nil {
	//		return "", err
	//	}

	//	var gitCommit GitCommit
	//	err = json.Unmarshal(queryResponse.Value, &gitCommit)
	//	if err != nil {
	//		return "", err
	//	}
	///
	//	if gitCommit.Repository == repository {
	//		lastCommit = &gitCommit
	//	}
	//}

	//if lastCommit == nil {
	//	return "", fmt.Errorf("no commits found for repository %s", repository)
	//}

	// Update the last commit with the remote URL
	//lastCommit.RemoteURL = fmt.Sprintf("%s", remoteURL)
	// Construct the remote URL without the \tree\ part
	//remoteURLWithHash := fmt.Sprintf("%s", remoteURL)

	//gitCommitJSON, err := json.Marshal(lastCommit)
	//if err != nil {
	//	return "", err
	//}

	//err = ctx.GetStub().PutState(lastCommit.CommitHash, gitCommitJSON)
	//if err != nil {
	//	return "", err
	//}

	// Store the push transaction
	repoVersion, err := s.GetRepositoryVersion(ctx, repository)
	if err != nil {
		return "", err
	}
	pushTx := PushTransaction{
		Repository: repository,
		RemoteURL:  remoteURLWithHash,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Version:    repoVersion.VersionNumber,
		//CommitHash: lastCommit.CommitHash, // Add commit hash to the push transaction
		CommitHash: commitHash,
	}
	pushTxJSON, err := json.Marshal(pushTx)
	if err != nil {
		return "", err
	}
	pushTxKey := fmt.Sprintf("PUSH_%s_%s", repository, pushTx.Timestamp)
	err = ctx.GetStub().PutState(pushTxKey, pushTxJSON)
	if err != nil {
		return "", err
	}

	message := fmt.Sprintf("All testing is done and audit processes approved, move to build stage. Git clone link: %s", remoteURLWithHash)
	return message, nil
}

func (s *SmartContract) GetAllPushTransactions(ctx contractapi.TransactionContextInterface) ([]*PushTransaction, error) {
	resultsIterator, err := ctx.GetStub().GetStateByRange("PUSH_", "PUSH_~")
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	var pushTransactions []*PushTransaction
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}

		var pushTx PushTransaction
		err = json.Unmarshal(queryResponse.Value, &pushTx)
		if err != nil {
			return nil, err
		}
		pushTransactions = append(pushTransactions, &pushTx)
	}
	// Format the output in a readable JSON format
	prettyResult, err := json.MarshalIndent(pushTransactions, "", "    ")
	if err != nil {
		return nil, fmt.Errorf("failed to format result: %v", err)
	}

	fmt.Printf("GetAllPushTransactions transaction successfully evaluated, result:\n%s\n", string(prettyResult))

	return pushTransactions, nil
}

// GetEvaluateTransactions returns functions of ComplexContract not to be tagged as submit
func (s *SmartContract) GetEvaluateTransactions() []string {
	return []string{"ReadGitCommit"}
}
