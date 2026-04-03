package acl

import "novpn/pkg/model"

type Evaluator interface {
	Allow(identity model.Identity, target model.TargetInfo) (model.Decision, error)
}

type AllowAllEvaluator struct{}

func (AllowAllEvaluator) Allow(_ model.Identity, _ model.TargetInfo) (model.Decision, error) {
	return model.Decision{
		Allowed: true,
		Reason:  "poc_allow_all",
	}, nil
}
