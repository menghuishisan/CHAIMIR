package sandbox

import (
	"context"
	"encoding/json"
	"strings"

	"chaimir/internal/contracts"
	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/jsonx"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/tenant"
	"chaimir/pkg/apperr"
	"chaimir/pkg/jsonvalue"
)

// PrepareCompositionClone 仅改写已登记快照中的素材，不重新解析最新镜像目录。
func (s *Service) PrepareCompositionClone(ctx context.Context, sourceTenantID int64, snapshot contracts.SandboxCompositionSnapshot, initCodeRef, initScriptRef string) (plan contracts.SandboxCompositionClonePlan, err error) {
	id, ok := tenant.FromContext(ctx)
	if !ok || id.TenantID <= 0 || id.AccountID <= 0 || id.IsPlatform || id.IsSystem || sourceTenantID <= 0 {
		return plan, apperr.ErrSandboxCreateRequestInvalid
	}
	digest, err := contracts.CanonicalSnapshotDigest(snapshot)
	if err != nil || digest != snapshot.Digest {
		return plan, apperr.ErrSandboxCreateRequestInvalid.WithCause(err)
	}
	if err = s.store.PlatformTx(ctx, func(ctx context.Context, tx TxStore) error {
		raw, err := tx.GetCompositionSnapshot(ctx, digest)
		if err != nil {
			return err
		}
		var stored contracts.SandboxCompositionSnapshot
		if err := jsonx.DecodeStrictKnownFields(raw, &stored); err != nil {
			return err
		}
		if !jsonvalue.Equal(snapshot, stored) {
			return apperr.ErrSandboxCreateRequestInvalid
		}
		return nil
	}); err != nil {
		return plan, err
	}
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return plan, err
	}
	value, err := jsonx.ObjectMapStrict(encoded)
	if err != nil {
		return plan, err
	}
	prefix, err := storage.ObjectKey(id.TenantID, "sandbox", "clone", ids.Format(s.ids.Generate()))
	if err != nil {
		return plan, err
	}
	copies, err := storage.NewCopyBatch(s.objectStorage, prefix, s.cfg.InitArchiveMaxBytes)
	if err != nil {
		return plan, err
	}
	plan.Discard = copies.Discard
	copyRef := func(ref storage.ObjectRef) (string, error) {
		if ref.Bucket != s.objectStorage.BucketCode() && ref.Bucket != s.objectStorage.BucketAttach() {
			return "", apperr.ErrSandboxInitObjectRefInvalid
		}
		if strings.HasPrefix(ref.Key, "0/") {
			return storage.ObjectRefString(ref.Bucket, ref.Key)
		}
		if err := validateInitObjectRef(sourceTenantID, ref, s.objectStorage.BucketCode(), s.objectStorage.BucketAttach()); err != nil {
			return "", err
		}
		return copies.Copy(ctx, ref)
	}
	if _, err = storage.RewriteObjectRefs(value, copyRef); err != nil {
		return plan, err
	}
	encoded, err = json.Marshal(value)
	if err != nil {
		return plan, err
	}
	if err = jsonx.DecodeStrictKnownFields(encoded, &plan.Snapshot); err != nil {
		return plan, err
	}
	for _, pair := range []struct {
		source string
		target *string
	}{{initCodeRef, &plan.InitCodeRef}, {initScriptRef, &plan.InitScriptRef}} {
		if pair.source == "" {
			continue
		}
		ref, err := storage.ParseObjectRef(pair.source)
		if err != nil {
			return plan, err
		}
		// 工作区代码和脚本没有平台素材例外。
		if err := validateInitObjectRef(sourceTenantID, ref, s.objectStorage.BucketCode(), s.objectStorage.BucketAttach()); err != nil {
			return plan, err
		}
		*pair.target, err = copyRef(ref)
		if err != nil {
			return plan, err
		}
	}
	plan.Snapshot.Digest, err = contracts.CanonicalSnapshotDigest(plan.Snapshot)
	if err != nil {
		return plan, err
	}
	raw, err := json.Marshal(plan.Snapshot)
	if err != nil {
		return plan, err
	}
	plan.Save = func(ctx context.Context) error {
		return s.store.CloneTx(ctx, id.TenantID, func(ctx context.Context, tx TxStore) error {
			return tx.UpsertCompositionSnapshot(ctx, plan.Snapshot.Digest, raw)
		})
	}
	return plan, nil
}

var _ contracts.SandboxCompositionCloneService = (*Service)(nil)
