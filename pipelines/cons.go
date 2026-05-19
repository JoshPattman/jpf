package pipelines

import "github.com/JoshPattman/jpf"

type ConstructionOpt[T, U any] func(*ConstructionKwargs[T, U])

type ConstructionKwargs[T, U any] struct {
	OutputFormat any
	Validator    jpf.Validator[T, U]
}

func GetConstructionKwargs[T, U any](opts ...ConstructionOpt[T, U]) ConstructionKwargs[T, U] {
	kw := &ConstructionKwargs[T, U]{}
	for _, o := range opts {
		o(kw)
	}
	return *kw
}

func WithOutputFormat[T, U any](obj any) ConstructionOpt[T, U] {
	return func(ck *ConstructionKwargs[T, U]) {
		ck.OutputFormat = obj
	}
}

func WithDefualtOutputFormat[T, U any]() ConstructionOpt[T, U] {
	return WithOutputFormat[T, U](*new(U))
}

func WithValidator[T, U any](validator jpf.Validator[T, U]) ConstructionOpt[T, U] {
	return func(ck *ConstructionKwargs[T, U]) {
		ck.Validator = validator
	}
}
