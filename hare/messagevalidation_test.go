package hare

import (
	"errors"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/signing"
	"github.com/stretchr/testify/assert"
	"testing"
)

func defaultValidator() *syntaxContextValidator {
	return newSyntaxContextValidator(signing.NewEdSigner(), lowThresh10, func(m *Msg) bool {
		return true
	}, &MockStateQuerier{true, nil}, 10, log.NewDefault("Validator"))
}

func TestMessageValidator_CommitStatus(t *testing.T) {
	assert.True(t, validateCommitType(BuildCommitMsg(generateSigning(t), NewEmptySet(lowDefaultSize))))
	assert.True(t, validateStatusType(BuildStatusMsg(generateSigning(t), NewEmptySet(lowDefaultSize))))
}

func TestMessageValidator_ValidateCertificate(t *testing.T) {
	validator := defaultValidator()
	assert.False(t, validator.validateCertificate(nil))
	cert := &Certificate{}
	assert.False(t, validator.validateCertificate(cert))
	cert.AggMsgs = &AggregatedMessages{}
	assert.False(t, validator.validateCertificate(cert))
	msgs := make([]*Message, 0, validator.threshold)
	cert.AggMsgs.Messages = msgs
	assert.False(t, validator.validateCertificate(cert))
	msgs = append(msgs, &Message{})
	cert.AggMsgs.Messages = msgs
	assert.False(t, validator.validateCertificate(cert))
	cert.Values = NewSetFromValues(value1).ToSlice()
	assert.False(t, validator.validateCertificate(cert))

	msgs = make([]*Message, validator.threshold)
	for i := 0; i < validator.threshold; i++ {
		msgs[i] = BuildCommitMsg(generateSigning(t), NewSmallEmptySet()).Message
	}
	cert.AggMsgs.Messages = msgs
	assert.True(t, validator.validateCertificate(cert))
}

func TestEligibilityValidator_validateRole(t *testing.T) {
	oracle := &mockRolacle{}
	ev := NewEligibilityValidator(oracle, 10, &mockIdProvider{}, 1, 5, log.NewDefault(""))
	ev.oracle = oracle
	res, err := ev.validateRole(nil)
	assert.NotNil(t, err)
	assert.False(t, res)
	m := BuildPreRoundMsg(generateSigning(t), NewSmallEmptySet())
	m.InnerMsg = nil
	res, err = ev.validateRole(m)
	assert.NotNil(t, err)
	assert.False(t, res)
	m = BuildPreRoundMsg(generateSigning(t), NewSmallEmptySet())
	oracle.isEligible = false
	res, err = ev.validateRole(m)
	assert.Nil(t, err)
	// TODO: remove comment after inceptions problem is addressed
	//assert.False(t, res)

	m.InnerMsg.InstanceId = 111
	myErr := errors.New("my error")
	ev.identityProvider = &mockIdProvider{myErr}
	res, err = ev.validateRole(m)
	assert.NotNil(t, err)
	assert.Equal(t, myErr, err)
	assert.False(t, res)

	oracle.err = myErr
	res, err = ev.validateRole(m)
	assert.NotNil(t, err)
	assert.Equal(t, myErr, err)
	assert.False(t, res)

	ev.identityProvider = &mockIdProvider{nil}
	oracle.err = nil
	res, err = ev.validateRole(m)
	assert.Nil(t, err)
	assert.False(t, res)

	oracle.isEligible = true
	m.InnerMsg.InstanceId = 111
	res, err = ev.validateRole(m)
	assert.Nil(t, err)
	assert.True(t, res)
}

func TestMessageValidator_IsStructureValid(t *testing.T) {
	validator := defaultValidator()
	assert.False(t, validator.SyntacticallyValidateMessage(nil))
	m := &Msg{&Message{}, nil}
	assert.False(t, validator.SyntacticallyValidateMessage(m))
	m.PubKey = generateSigning(t).PublicKey()
	assert.False(t, validator.SyntacticallyValidateMessage(m))
	m.InnerMsg = &InnerMessage{}
	assert.False(t, validator.SyntacticallyValidateMessage(m))
	m.InnerMsg.Values = nil
	assert.False(t, validator.SyntacticallyValidateMessage(m))
	m.InnerMsg.Values = NewSmallEmptySet().ToSlice()
	assert.False(t, validator.SyntacticallyValidateMessage(m))
}

func TestMessageValidator_Aggregated(t *testing.T) {
	validator := defaultValidator()
	assert.False(t, validator.validateAggregatedMessage(nil, nil))
	funcs := make([]func(m *Msg) bool, 0)
	assert.False(t, validator.validateAggregatedMessage(nil, funcs))

	agg := &AggregatedMessages{}
	assert.False(t, validator.validateAggregatedMessage(agg, funcs))
	msgs := make([]*Message, validator.threshold)
	for i := 0; i < validator.threshold; i++ {
		iMsg := BuildStatusMsg(generateSigning(t), NewSetFromValues(value1))
		msgs[i] = iMsg.Message
	}
	agg.Messages = msgs
	assert.True(t, validator.validateAggregatedMessage(agg, funcs))
	msgs[0].Sig = []byte{1}
	assert.False(t, validator.validateAggregatedMessage(agg, funcs))

	funcs = make([]func(m *Msg) bool, 1)
	funcs[0] = func(m *Msg) bool { return false }
	assert.False(t, validator.validateAggregatedMessage(agg, funcs))
}

func TestSyntaxContextValidator_PreRoundContext(t *testing.T) {
	validator := defaultValidator()
	ed := signing.NewEdSigner()
	for i := 0; i < 10; i++ {
		res, e := validator.ContextuallyValidateMessage(BuildPreRoundMsg(ed, NewSmallEmptySet()), int32(i))
		assertNoErr(t, true, res, e)
	}
}

func TestSyntaxContextValidator_NotifyContext(t *testing.T) {
	validator := defaultValidator()
	ed := signing.NewEdSigner()
	for i := 0; i < 10; i++ {
		res, e := validator.ContextuallyValidateMessage(BuildNotifyMsg(ed, NewSmallEmptySet()), int32(i))
		assertNoErr(t, true, res, e)
	}
}

func TestSyntaxContextValidator_StatusContext(t *testing.T) {
	validator := defaultValidator()
	ed := signing.NewEdSigner()
	_, e := validator.ContextuallyValidateMessage(BuildStatusMsg(ed, NewSmallEmptySet()), -1)
	assert.Error(t, e, errEarlyMsg.Error())

	results := []bool{true, false, false, false}
	for i := 0; i < 4; i++ {
		res, e := validator.ContextuallyValidateMessage(BuildStatusMsg(ed, NewSmallEmptySet()), int32(i))
		assertNoErr(t, results[i], res, e)
	}
}

func TestSyntaxContextValidator_ProposalContext(t *testing.T) {
	validator := defaultValidator()
	ed := signing.NewEdSigner()
	res, e := validator.ContextuallyValidateMessage(BuildProposalMsg(ed, NewSmallEmptySet()), -1)
	assertNoErr(t, false, res, e)

	res, e = validator.ContextuallyValidateMessage(BuildProposalMsg(ed, NewSmallEmptySet()), 0)
	assert.Error(t, e, errEarlyMsg.Error())

	res, e = validator.ContextuallyValidateMessage(BuildProposalMsg(ed, NewSmallEmptySet()), 1)
	assertNoErr(t, true, res, e)

	res, e = validator.ContextuallyValidateMessage(BuildProposalMsg(ed, NewSmallEmptySet()), 2)
	assertNoErr(t, true, res, e)

	res, e = validator.ContextuallyValidateMessage(BuildProposalMsg(ed, NewSmallEmptySet()), 3)
	assertNoErr(t, false, res, e)
}

func TestSyntaxContextValidator_CommitContext(t *testing.T) {
	validator := defaultValidator()
	ed := signing.NewEdSigner()
	res, e := validator.ContextuallyValidateMessage(BuildCommitMsg(ed, NewSmallEmptySet()), -1)
	assertNoErr(t, false, res, e)

	res, e = validator.ContextuallyValidateMessage(BuildCommitMsg(ed, NewSmallEmptySet()), 0)
	assertNoErr(t, false, res, e)

	res, e = validator.ContextuallyValidateMessage(BuildCommitMsg(ed, NewSmallEmptySet()), 1)
	assert.Error(t, e, errEarlyMsg.Error())

	res, e = validator.ContextuallyValidateMessage(BuildCommitMsg(ed, NewSmallEmptySet()), 2)
	assertNoErr(t, true, res, e)

	res, e = validator.ContextuallyValidateMessage(BuildCommitMsg(ed, NewSmallEmptySet()), 3)
	assertNoErr(t, false, res, e)
}

func TestMessageValidator_ValidateMessage(t *testing.T) {
	proc := generateConsensusProcess(t)
	proc.advanceToNextRound()
	v := proc.validator
	b, err := proc.initDefaultBuilder(proc.s)
	assert.Nil(t, err)
	preround := b.SetType(PreRound).Sign(proc.signing).Build()
	preround.PubKey = proc.signing.PublicKey()
	assert.True(t, v.SyntacticallyValidateMessage(preround))
	res, e := v.ContextuallyValidateMessage(preround, 0)
	assert.NoError(t, e)
	assert.True(t, res)
	b, err = proc.initDefaultBuilder(proc.s)
	assert.Nil(t, err)
	status := b.SetType(Status).Sign(proc.signing).Build()
	status.PubKey = proc.signing.PublicKey()
	res, e = v.ContextuallyValidateMessage(status, 0)
	assert.NoError(t, e)
	assert.True(t, res)
	assert.True(t, v.SyntacticallyValidateMessage(status))

}

func assertNoErr(t *testing.T, expect bool, actual bool, err error) {
	assert.NoError(t, err)
	assert.Equal(t, expect, actual)
}

func TestMessageValidator_SyntacticallyValidateMessage(t *testing.T) {
	validator := newSyntaxContextValidator(signing.NewEdSigner(), 1, validate, &MockStateQuerier{true, nil}, 10, log.NewDefault("Validator"))
	m := BuildPreRoundMsg(generateSigning(t), NewSmallEmptySet())
	assert.False(t, validator.SyntacticallyValidateMessage(m))
	m = BuildPreRoundMsg(generateSigning(t), NewSetFromValues(value1))
	assert.True(t, validator.SyntacticallyValidateMessage(m))
}

func TestMessageValidator_ContextuallyValidateMessage(t *testing.T) {
	validator := newSyntaxContextValidator(signing.NewEdSigner(), 1, validate, &MockStateQuerier{true, nil}, 10, log.NewDefault("Validator"))
	m := BuildPreRoundMsg(generateSigning(t), NewSmallEmptySet())
	m.InnerMsg = nil
	res, e := validator.ContextuallyValidateMessage(m, 0)
	assert.Error(t, e)
	m = BuildPreRoundMsg(generateSigning(t), NewSetFromValues(value1))
	res, e = validator.ContextuallyValidateMessage(m, 0)
	assertNoErr(t, true, res, e)
	m = BuildStatusMsg(generateSigning(t), NewSetFromValues(value1))
	res, e = validator.ContextuallyValidateMessage(m, 1)
	assertNoErr(t, false, res, e)
	res, e = validator.ContextuallyValidateMessage(m, 0)
	assertNoErr(t, true, res, e)
}

func TestMessageValidator_validateSVPTypeA(t *testing.T) {
	m := buildProposalMsg(signing.NewEdSigner(), NewSetFromValues(value1, value2, value3), []byte{})
	s1 := NewSetFromValues(value1)
	s2 := NewSetFromValues(value3)
	s3 := NewSetFromValues(value1, value5)
	s4 := NewSetFromValues(value1, value4)
	v := defaultValidator()
	m.InnerMsg.Svp = buildSVP(-1, s1, s2, s3, s4)
	assert.False(t, v.validateSVPTypeA(m))
	s3 = NewSetFromValues(value2)
	m.InnerMsg.Svp = buildSVP(-1, s1, s2, s3)
	assert.True(t, v.validateSVPTypeA(m))
}

func TestMessageValidator_validateSVPTypeB(t *testing.T) {
	m := buildProposalMsg(signing.NewEdSigner(), NewSetFromValues(value1, value2, value3), []byte{})
	s1 := NewSetFromValues(value1)
	m.InnerMsg.Svp = buildSVP(-1, s1)
	s := NewSetFromValues(value1)
	m.InnerMsg.Values = s.ToSlice()
	v := defaultValidator()
	assert.False(t, v.validateSVPTypeB(m, NewSetFromValues(value5)))
	assert.True(t, v.validateSVPTypeB(m, NewSetFromValues(value1)))
}

func TestMessageValidator_validateSVP(t *testing.T) {
	validator := newSyntaxContextValidator(signing.NewEdSigner(), 1, validate, &MockStateQuerier{true, nil}, 10, log.NewDefault("Validator"))
	m := buildProposalMsg(signing.NewEdSigner(), NewSetFromValues(value1, value2, value3), []byte{})
	s1 := NewSetFromValues(value1)
	m.InnerMsg.Svp = buildSVP(-1, s1)
	m.InnerMsg.Svp.Messages[0].InnerMsg.Type = Commit
	assert.False(t, validator.validateSVP(m))
	m.InnerMsg.Svp = buildSVP(-1, s1)
	m.InnerMsg.Svp.Messages[0].InnerMsg.K = 4
	assert.False(t, validator.validateSVP(m))
	m.InnerMsg.Svp = buildSVP(-1, s1)
	assert.False(t, validator.validateSVP(m))
	s2 := NewSetFromValues(value1, value2, value3)
	m.InnerMsg.Svp = buildSVP(-1, s2)
	assert.True(t, validator.validateSVP(m))
	m.InnerMsg.Svp = buildSVP(0, s1)
	assert.False(t, validator.validateSVP(m))
	m.InnerMsg.Svp = buildSVP(0, s2)
	assert.True(t, validator.validateSVP(m))
}

func buildSVP(ki int32, S ...*Set) *AggregatedMessages {
	msgs := make([]*Message, 0, len(S))
	for _, s := range S {
		msgs = append(msgs, buildStatusMsg(signing.NewEdSigner(), s, ki).Message)
	}

	svp := &AggregatedMessages{}
	svp.Messages = msgs
	return svp
}
