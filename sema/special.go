package sema

import (
	"github.com/vertex-language/vcx/types"
)

// SynthesizeSpecialMembers declares the implicit special member functions for a record.
func SynthesizeSpecialMembers(r *types.Record) {
	if r == nil || !r.Complete {
		return
	}

	hasAnyCtor := false
	hasCopyCtor := false
	hasMoveCtor := false
	hasCopyAssign := false
	hasMoveAssign := false
	hasDtor := false

	refType := &types.LValueReference{Elem: r}
	constRefType := &types.LValueReference{Elem: types.Qualify(r, types.QConst)}
	rvalueRefType := &types.RValueReference{Elem: r}

	for _, m := range r.Methods {
		if m.Name == r.Name {
			hasAnyCtor = true
			if len(m.Func.Params) == 1 {
				pt := m.Func.Params[0].Type
				if pt.Equal(constRefType) || pt.Equal(refType) {
					hasCopyCtor = true
				} else if pt.Equal(rvalueRefType) {
					hasMoveCtor = true
				}
			}
		}
		if m.Name == "~"+r.Name {
			hasDtor = true
		}
		if m.Name == "operator=" && len(m.Func.Params) == 1 {
			pt := m.Func.Params[0].Type
			if pt.Equal(constRefType) || pt.Equal(refType) {
				hasCopyAssign = true
			} else if pt.Equal(rvalueRefType) {
				hasMoveAssign = true
			}
		}
	}

	// 1. Implicit Default Constructor: C()
	if !hasAnyCtor {
		defaultCtor := &types.Method{
			Name: r.Name,
			Func: &types.Func{
				Ret:      types.Typ(types.Void),
				Params:   nil,
				Noexcept: true,
			},
			Access:    types.AccessPublic,
			Defaulted: true,
		}
		r.Methods = append(r.Methods, defaultCtor)
	}

	// 2. Implicit Copy Constructor: C(const C&)
	if !hasCopyCtor {
		copyCtor := &types.Method{
			Name: r.Name,
			Func: &types.Func{
				Ret: types.Typ(types.Void),
				Params: []types.Param{
					{Name: "other", Type: constRefType},
				},
			},
			Access:    types.AccessPublic,
			Defaulted: true,
		}
		r.Methods = append(r.Methods, copyCtor)
	}

	// 3. Implicit Move Constructor: C(C&&)
	// Declared if no user-declared copy ctor, copy assign, move assign, or dtor
	if !hasMoveCtor && !hasCopyCtor && !hasCopyAssign && !hasMoveAssign && !hasDtor {
		moveCtor := &types.Method{
			Name: r.Name,
			Func: &types.Func{
				Ret: types.Typ(types.Void),
				Params: []types.Param{
					{Name: "other", Type: rvalueRefType},
				},
				Noexcept: true,
			},
			Access:    types.AccessPublic,
			Defaulted: true,
		}
		r.Methods = append(r.Methods, moveCtor)
	}

	// 4. Implicit Copy Assignment: C& operator=(const C&)
	if !hasCopyAssign {
		copyAssign := &types.Method{
			Name: "operator=",
			Func: &types.Func{
				Ret: refType,
				Params: []types.Param{
					{Name: "other", Type: constRefType},
				},
			},
			Access:    types.AccessPublic,
			Defaulted: true,
		}
		r.Methods = append(r.Methods, copyAssign)
	}

	// 5. Implicit Move Assignment: C& operator=(C&&)
	if !hasMoveAssign && !hasCopyCtor && !hasCopyAssign && !hasMoveCtor && !hasDtor {
		moveAssign := &types.Method{
			Name: "operator=",
			Func: &types.Func{
				Ret: refType,
				Params: []types.Param{
					{Name: "other", Type: rvalueRefType},
				},
				Noexcept: true,
			},
			Access:    types.AccessPublic,
			Defaulted: true,
		}
		r.Methods = append(r.Methods, moveAssign)
	}

	// 6. Implicit Destructor: ~C()
	if !hasDtor {
		dtor := &types.Method{
			Name: "~" + r.Name,
			Func: &types.Func{
				Ret:      types.Typ(types.Void),
				Noexcept: true,
			},
			Access:    types.AccessPublic,
			Defaulted: true,
		}
		r.Methods = append(r.Methods, dtor)
	}
}
