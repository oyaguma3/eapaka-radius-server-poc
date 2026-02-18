package policy

import "errors"

var (
	// ErrPolicyNotFound はポリシーが見つからない場合のエラー
	ErrPolicyNotFound = errors.New("policy not found")

	// ErrPolicyInvalid はポリシーの内容が不正な場合のエラー
	ErrPolicyInvalid = errors.New("policy invalid")

	// ErrPolicyDenied はポリシー評価の結果アクセスが拒否された場合のエラー
	ErrPolicyDenied = errors.New("policy denied")
)
