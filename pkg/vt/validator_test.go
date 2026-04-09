package vt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	colorGray  = "gray"
	colorBrown = "brown"

	circusNameGood = "Amazing Circus"
	circusNameBad  = "xxxxxxxxxxxxxxxxx"

	animalNameOne   = "Cecile"
	animalNameTwo   = "Shipa"
	animalNameThree = "Raymonda"
	animalNameDummy = "Dummy"
	animalNameBad   = "xxxxxxxxxxxxx"
)

type Circus struct {
	Name    string   `json:"name" validate:"required,min=3,max=16"`
	Animals []Animal `json:"animals" validate:"required,dive"`
}

type Animal struct {
	Name   string `json:"name" validate:"required,min=3,max=12"`
	Weight int    `json:"weight" validate:"required,gt=0,lt=50"`
	Tail   *Tail  `json:"optionalTail"`
}

type Tail struct {
	Color  string `json:"color" validate:"required"`
	Length int    `json:"length" validate:"gt=0"`
}

func TestValidator_CheckBasic(t *testing.T) {
	var (
		tailOne   = Tail{Color: colorBrown, Length: 31}
		tailTwo   = Tail{Color: colorGray, Length: 35}
		tailThree = Tail{Color: colorBrown, Length: 28}
		tailBad   = Tail{}

		animalOne   = Animal{Name: animalNameOne, Weight: 5, Tail: &tailOne}
		animalTwo   = Animal{Name: animalNameTwo, Weight: 4, Tail: &tailTwo}
		animalThree = Animal{Name: animalNameThree, Weight: 6, Tail: &tailThree}
		animalDummy = Animal{Name: animalNameDummy, Weight: 6}
		animalBad   = Animal{Name: animalNameBad, Weight: 500, Tail: &tailBad}
	)

	t.Run("Positive case", func(t *testing.T) {
		var v Validator
		ctx := t.Context()

		v.CheckBasic(ctx, &Circus{
			Name:    circusNameGood,
			Animals: []Animal{animalOne, animalTwo, animalThree, animalDummy},
		})
		assert.False(t, v.HasErrors())
		assert.Empty(t, v.Fields())
	})

	t.Run("Negative cases", func(t *testing.T) {
		t.Run("full", func(t *testing.T) {
			var v Validator
			ctx := t.Context()

			v.CheckBasic(ctx, &Circus{
				Name:    circusNameBad,
				Animals: []Animal{animalBad},
			})
			assert.True(t, v.HasErrors())
			assert.Len(t, v.Fields(), 5)

			assert.Contains(t, v.Fields(), FieldError{Field: "name", Error: "max", Constraint: &FieldErrorConstraint{Max: 16}})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[0].name", Error: "max", Constraint: &FieldErrorConstraint{Max: 12}})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[0].weight", Error: "lt"})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[0].optionalTail.color", Error: "required"})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[0].optionalTail.length", Error: "required"})
		})

		t.Run("good circus name", func(t *testing.T) {
			var v Validator
			ctx := t.Context()

			v.CheckBasic(ctx, &Circus{
				Name:    circusNameGood,
				Animals: []Animal{animalBad},
			})
			assert.True(t, v.HasErrors())
			assert.Len(t, v.Fields(), 4)

			assert.Contains(t, v.Fields(), FieldError{Field: "animals[0].name", Error: "max", Constraint: &FieldErrorConstraint{Max: 12}})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[0].weight", Error: "lt"})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[0].optionalTail.color", Error: "required"})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[0].optionalTail.length", Error: "required"})
		})

		t.Run("good circus name, [animalBad, animalOne]", func(t *testing.T) {
			var v Validator
			ctx := t.Context()

			v.CheckBasic(ctx, &Circus{
				Name:    circusNameGood,
				Animals: []Animal{animalBad, animalOne},
			})
			assert.True(t, v.HasErrors())
			assert.Len(t, v.Fields(), 4)

			assert.Contains(t, v.Fields(), FieldError{Field: "animals[0].name", Error: "max", Constraint: &FieldErrorConstraint{Max: 12}})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[0].weight", Error: "lt"})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[0].optionalTail.color", Error: "required"})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[0].optionalTail.length", Error: "required"})
		})

		t.Run("good circus name, [animalOne, animalBad]", func(t *testing.T) {
			var v Validator
			ctx := t.Context()

			v.CheckBasic(ctx, &Circus{
				Name:    circusNameGood,
				Animals: []Animal{animalOne, animalBad},
			})
			assert.True(t, v.HasErrors())
			assert.Len(t, v.Fields(), 4)

			assert.Contains(t, v.Fields(), FieldError{Field: "animals[1].name", Error: "max", Constraint: &FieldErrorConstraint{Max: 12}})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[1].weight", Error: "lt"})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[1].optionalTail.color", Error: "required"})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[1].optionalTail.length", Error: "required"})
		})

		t.Run("good circus name, [animalOne, animalBad, animalDummy]", func(t *testing.T) {
			var v Validator
			ctx := t.Context()

			v.CheckBasic(ctx, &Circus{
				Name:    circusNameGood,
				Animals: []Animal{animalOne, animalBad, animalDummy},
			})
			assert.True(t, v.HasErrors())
			assert.Len(t, v.Fields(), 4)

			assert.Contains(t, v.Fields(), FieldError{Field: "animals[1].name", Error: "max", Constraint: &FieldErrorConstraint{Max: 12}})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[1].weight", Error: "lt"})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[1].optionalTail.color", Error: "required"})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[1].optionalTail.length", Error: "required"})
		})

		t.Run("good circus name, [animalOne, animalBad, animalTwo, animalBad]", func(t *testing.T) {
			var v Validator
			ctx := t.Context()

			v.CheckBasic(ctx, &Circus{
				Name:    circusNameGood,
				Animals: []Animal{animalOne, animalBad, animalTwo, animalBad},
			})
			assert.True(t, v.HasErrors())
			assert.Len(t, v.Fields(), 8)

			assert.Contains(t, v.Fields(), FieldError{Field: "animals[1].name", Error: "max", Constraint: &FieldErrorConstraint{Max: 12}})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[1].weight", Error: "lt"})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[1].optionalTail.color", Error: "required"})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[1].optionalTail.length", Error: "required"})

			assert.Contains(t, v.Fields(), FieldError{Field: "animals[3].name", Error: "max", Constraint: &FieldErrorConstraint{Max: 12}})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[3].weight", Error: "lt"})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[3].optionalTail.color", Error: "required"})
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[3].optionalTail.length", Error: "required"})
		})

		t.Run("good circus name, no animals", func(t *testing.T) {
			var v Validator
			ctx := t.Context()

			v.CheckBasic(ctx, &Circus{
				Name: circusNameGood,
			})
			assert.True(t, v.HasErrors())
			assert.Len(t, v.Fields(), 1)
			assert.Contains(t, v.Fields(), FieldError{Field: "animals", Error: "required"})
		})

		t.Run("good circus name, one overweighted animal", func(t *testing.T) {
			var v Validator
			ctx := t.Context()

			v.CheckBasic(ctx, &Circus{
				Name:    circusNameGood,
				Animals: []Animal{{Name: animalNameOne, Weight: 200}},
			})
			assert.True(t, v.HasErrors())
			assert.Len(t, v.Fields(), 1)
			assert.Contains(t, v.Fields(), FieldError{Field: "animals[0].weight", Error: "lt"})
		})
	})
}
