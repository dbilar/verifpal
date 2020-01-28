/* SPDX-FileCopyrightText: © 2019-2020 Nadim Kobeissi <nadim@symbolic.software>
 * SPDX-License-Identifier: GPL-3.0-only */
// bc668866bf7ad5972a2f8a9999e62fe7

package verifpal

import (
	"fmt"
	"sync"
	"sync/atomic"
)

var verifyAnalysisCount uint32

func verifyAnalysis(valKnowledgeMap knowledgeMap, valPrincipalState principalState, stage int, sg *sync.WaitGroup) {
	var aGroup sync.WaitGroup
	var pGroup sync.WaitGroup
	var o uint32
	valAttackerState := attackerStateGetRead()
	for _, a := range valAttackerState.known {
		aGroup.Add(1)
		go func(a value) {
			switch a.kind {
			case "constant":
				a = sanityResolveConstant(a.constant, valPrincipalState)
			}
			atomic.AddUint32(&o, verifyAnalysisDecompose(a, valPrincipalState, valAttackerState, 0))
			atomic.AddUint32(&o, verifyAnalysisEquivalize(a, valPrincipalState, 0))
			atomic.AddUint32(&o, verifyAnalysisPasswords(a, valPrincipalState, 0))
			aGroup.Done()
		}(a)
	}
	for _, a := range valPrincipalState.assigned {
		pGroup.Add(1)
		go func(a value) {
			atomic.AddUint32(&o, verifyAnalysisRecompose(a, valPrincipalState, valAttackerState, 0))
			atomic.AddUint32(&o, verifyAnalysisReconstruct(a, valPrincipalState, valAttackerState, 0))
			pGroup.Done()
		}(a)
	}
	aGroup.Wait()
	pGroup.Wait()
	verifyResolveQueries(valKnowledgeMap, valPrincipalState, valAttackerState)
	verifyAnalysisIncrementCount()
	prettyAnalysis(stage)
	if atomic.LoadUint32(&o) > 0 {
		sg.Add(1)
		go verifyAnalysis(valKnowledgeMap, valPrincipalState, stage, sg)
	}
	sg.Done()
}

func verifyAnalysisIncrementCount() {
	atomic.AddUint32(&verifyAnalysisCount, 1)
}

func verifyAnalysisGetCount() int {
	return int(atomic.LoadUint32(&verifyAnalysisCount))
}

func verifyAnalysisDecompose(
	a value, valPrincipalState principalState, valAttackerState attackerState, o uint32,
) uint32 {
	var r bool
	var revealed value
	var ar []value
	oo := o
	switch a.kind {
	case "primitive":
		r, revealed, ar = possibleToDecomposePrimitive(a.primitive, valPrincipalState, valAttackerState)
	}
	if r {
		write := attackerStateWrite{
			known:     revealed,
			wire:      false,
			mutatedTo: []string{},
		}
		if attackerStatePutWrite(write) {
			prettyMessage(fmt.Sprintf(
				"%s obtained by decomposing %s with %s.",
				prettyValue(revealed), prettyValue(a), prettyValues(ar),
			), "deduction", true)
			o = o + 1
		}
	}
	if o > oo {
		return verifyAnalysisDecompose(a, valPrincipalState, valAttackerState, o)
	}
	return o
}

func verifyAnalysisRecompose(
	a value, valPrincipalState principalState, valAttackerState attackerState, o uint32,
) uint32 {
	var r bool
	var revealed value
	var ar []value
	oo := o
	switch a.kind {
	case "primitive":
		r, revealed, ar = possibleToRecomposePrimitive(a.primitive, valPrincipalState, valAttackerState)
	}
	if r {
		write := attackerStateWrite{
			known:     revealed,
			wire:      false,
			mutatedTo: []string{},
		}
		if attackerStatePutWrite(write) {
			prettyMessage(fmt.Sprintf(
				"%s obtained by recomposing %s with %s.",
				prettyValue(revealed), prettyValue(a), prettyValues(ar),
			), "deduction", true)
			o = o + 1
		}
	}
	if o > oo {
		return verifyAnalysisRecompose(a, valPrincipalState, valAttackerState, o)
	}
	return o
}

func verifyAnalysisReconstruct(
	a value, valPrincipalState principalState, valAttackerState attackerState, o uint32,
) uint32 {
	var r bool
	var ar []value
	oo := o
	switch a.kind {
	case "primitive":
		r, ar = possibleToReconstructPrimitive(a.primitive, valPrincipalState, valAttackerState)
		for _, aa := range a.primitive.arguments {
			verifyAnalysisReconstruct(aa, valPrincipalState, valAttackerState, o)
		}
	case "equation":
		r, ar = possibleToReconstructEquation(a.equation, valPrincipalState, valAttackerState)
	}
	if r {
		write := attackerStateWrite{
			known:     a,
			wire:      false,
			mutatedTo: []string{},
		}
		if attackerStatePutWrite(write) {
			prettyMessage(fmt.Sprintf(
				"%s obtained by reconstructing with %s.",
				prettyValue(a), prettyValues(ar),
			), "deduction", true)
			o = o + 1
		}
	}
	if o > oo {
		return verifyAnalysisReconstruct(a, valPrincipalState, valAttackerState, o)
	}
	return o
}

func verifyAnalysisEquivalize(a value, valPrincipalState principalState, o uint32) uint32 {
	oo := o
	for _, c := range valPrincipalState.constants {
		aa := sanityResolveConstant(c, valPrincipalState)
		if sanityEquivalentValues(a, aa, valPrincipalState) {
			write := attackerStateWrite{
				known:     aa,
				wire:      false,
				mutatedTo: []string{},
			}
			if attackerStatePutWrite(write) {
				o = o + 1
			}
		}
		switch aa.kind {
		case "primitive":
			for _, aaa := range aa.primitive.arguments {
				if sanityEquivalentValues(a, aaa, valPrincipalState) {
					write := attackerStateWrite{
						known:     aaa,
						wire:      false,
						mutatedTo: []string{},
					}
					if attackerStatePutWrite(write) {
						o = o + 1
					}
				}
			}
		}
	}
	if o > oo {
		return verifyAnalysisEquivalize(a, valPrincipalState, o)
	}
	return o
}

func verifyAnalysisPasswords(a value, valPrincipalState principalState, o uint32) uint32 {
	oo := o
	passwords := possibleToObtainPasswords(a, valPrincipalState)
	for _, password := range passwords {
		write := attackerStateWrite{
			known:     password,
			wire:      false,
			mutatedTo: []string{},
		}
		if attackerStatePutWrite(write) {
			prettyMessage(fmt.Sprintf(
				"%s obtained as a password unsafely used within %s.",
				prettyValue(password), prettyValue(a),
			), "deduction", true)
			o = o + 1
		}
	}
	if o > oo {
		return verifyAnalysisPasswords(a, valPrincipalState, o)
	}
	return o
}
