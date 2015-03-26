// Copyright ©2014 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package optimize

import (
	"fmt"
	"math"
	"time"

	"github.com/gonum/floats"
	"github.com/gonum/matrix/mat64"
)

// Local finds a local minimum of a function using a sequential algorithm.
// In order to maximize a function, multiply the output by -1.
//
// The first argument is of Function type representing the function to be minimized.
// Type switching is used to see if the function implements Gradient, FunctionGradient
// and Statuser.
//
// The second argument is the initial location at which to start the minimization.
// The initial location must be supplied, and must have a length equal to the
// problem dimension.
//
// The third argument contains the settings for the minimization. It is here that
// gradient tolerance, etc. are specified. The DefaultSettings() function
// can be called for a Settings struct with the default values initialized.
// If settings == nil, the default settings are used. Please see the documentation
// for the Settings structure for more information. The optimization Method used
// may also contain settings, see documentation for the appropriate optimizer.
//
// The final argument is the optimization method to use. If method == nil, then
// an appropriate default is chosen based on the properties of the other arguments
// (dimension, gradient-free or gradient-based, etc.). The optimization
// methods in this package are designed such that reasonable defaults occur
// if options are not specified explicitly. For example, the code
//  method := &Bfgs{}
// creates a pointer to a new Bfgs struct. When minimize is called, the settings
// in the method will be populated with default values. The methods are also
// designed such that they can be reused in future calls to method.
//
// Local returns a Result struct and any error that occurred. Please see the
// documentation of Result for more information.
//
// Please be aware that the default behavior of Local is to find the minimum.
// For certain functions and optimization methods, this process can take many
// function evaluations. If you would like to put limits on this, for example
// maximum runtime or maximum function evaluations, please modify the Settings
// input struct.
func Local(f Function, initX []float64, settings *Settings, method Method) (*Result, error) {
	if len(initX) == 0 {
		panic("optimize: initial X has zero length")
	}

	startTime := time.Now()
	funcInfo := newFunctionInfo(f)
	if method == nil {
		method = getDefaultMethod(funcInfo)
	}
	if err := funcInfo.satisfies(method); err != nil {
		return nil, err
	}

	if funcInfo.IsStatuser {
		_, err := funcInfo.statuser.Status()
		if err != nil {
			return nil, err
		}
	}

	if settings == nil {
		settings = DefaultSettings()
	}

	if settings.Recorder != nil {
		// Initialize Recorder first. If it fails, we avoid the (possibly
		// time-consuming) evaluation of F and DF at the starting location.
		err := settings.Recorder.Init(&funcInfo.FunctionInfo)
		if err != nil {
			return nil, err
		}
	}

	stats := &Stats{}
	optLoc, evalType, err := getStartingLocation(funcInfo, method, initX, stats, settings)
	if err != nil {
		return nil, err
	}

	// Runtime is the only Stats field that needs to be updated here.
	stats.Runtime = time.Since(startTime)
	// Send optLoc to Recorder before checking it for convergence.
	if settings.Recorder != nil {
		err = settings.Recorder.Record(optLoc, evalType, InitIteration, stats)
	}

	// Check if the starting location satisfies the convergence criteria.
	status := checkConvergence(optLoc, InitIteration, stats, settings)
	if status == NotTerminated && err == nil {
		// The starting location is not good enough, we need to perform a
		// minimization. The optimal location will be stored in-place in
		// optLoc.
		status, err = minimize(settings, method, funcInfo, stats, optLoc, startTime)
	}

	if settings.Recorder != nil && err == nil {
		// Send the optimal location to Recorder.
		err = settings.Recorder.Record(optLoc, NoEvaluation, PostIteration, stats)
	}
	stats.Runtime = time.Since(startTime)
	return &Result{
		Location: *optLoc,
		Stats:    *stats,
		Status:   status,
	}, err
}

func minimize(settings *Settings, method Method, funcInfo *functionInfo, stats *Stats, optLoc *Location, startTime time.Time) (status Status, err error) {
	loc := &Location{}
	copyLocation(loc, optLoc)
	xNext := make([]float64, len(loc.X))

	methodStatus, methodIsStatuser := method.(Statuser)

	evalType, iterType, err := method.Init(loc, &funcInfo.FunctionInfo, xNext)
	if err != nil {
		return Failure, err
	}

	for {
		if funcInfo.IsStatuser {
			// Check the function status before evaluating.
			status, err = funcInfo.statuser.Status()
			if err != nil || status != NotTerminated {
				return
			}
		}

		// Perform evalType evaluation of the function at xNext and store the
		// result in location.
		evaluate(funcInfo, evalType, xNext, loc, stats)
		// Update the stats and optLoc.
		update(loc, optLoc, stats, iterType, startTime)
		// Get the convergence status before recording the new location.
		status = checkConvergence(optLoc, iterType, stats, settings)

		if settings.Recorder != nil {
			err = settings.Recorder.Record(loc, evalType, iterType, stats)
			if err != nil {
				if status == NotTerminated {
					status = Failure
				}
				return
			}
		}

		if status != NotTerminated {
			return
		}

		if methodIsStatuser {
			status, err = methodStatus.Status()
			if err != nil || status != NotTerminated {
				return
			}
		}

		// Find the next location (stored in-place into xNext).
		evalType, iterType, err = method.Iterate(loc, xNext)
		if err != nil {
			status = Failure
			return
		}
	}
	panic("optimize: unreachable")
}

func copyLocation(dst, src *Location) {
	dst.X = resize(dst.X, len(src.X))
	copy(dst.X, src.X)

	dst.F = src.F

	dst.Gradient = resize(dst.Gradient, len(src.Gradient))
	copy(dst.Gradient, src.Gradient)

	if src.Hessian != nil {
		if dst.Hessian == nil || dst.Hessian.Symmetric() != len(src.X) {
			dst.Hessian = mat64.NewSymDense(len(src.X), nil)
		}
		dst.Hessian.CopySym(src.Hessian)
	}
}

func getDefaultMethod(funcInfo *functionInfo) Method {
	if funcInfo.IsGradient || funcInfo.IsFunctionGradient {
		return &BFGS{}
	}
	// TODO: Implement a gradient-free method
	panic("optimize: gradient-free methods not yet coded")
}

// getStartingLocation allocates and initializes the starting location for the minimization.
func getStartingLocation(funcInfo *functionInfo, method Method, initX []float64, stats *Stats, settings *Settings) (*Location, EvaluationType, error) {
	dim := len(initX)
	loc := &Location{
		X: make([]float64, dim),
	}
	copy(loc.X, initX)
	if method.Needs().Gradient {
		loc.Gradient = make([]float64, dim)
	}
	if method.Needs().Hessian {
		loc.Hessian = mat64.NewSymDense(dim, nil)
	}

	evalType := NoEvaluation
	if settings.UseInitialData {
		loc.F = settings.InitialFunctionValue
		if loc.Gradient != nil {
			initG := settings.InitialGradient
			if len(initG) != dim {
				panic("optimize: initial location size mismatch")
			}
			copy(loc.Gradient, initG)
		}
	} else {
		evalType = FuncEvaluation
		if loc.Gradient != nil {
			evalType = FuncGradEvaluation
		}
		evaluate(funcInfo, evalType, loc.X, loc, stats)
	}

	if math.IsNaN(loc.F) {
		return loc, evalType, ErrNaN
	}
	if math.IsInf(loc.F, 1) {
		return loc, evalType, ErrInf
	}
	for _, v := range loc.Gradient {
		if math.IsInf(v, 0) {
			return loc, evalType, ErrGradInf
		}
		if math.IsNaN(v) {
			return loc, evalType, ErrGradNaN
		}
	}

	return loc, evalType, nil
}

func checkConvergence(loc *Location, iterType IterationType, stats *Stats, settings *Settings) Status {
	if iterType == MajorIteration || iterType == InitIteration {
		if loc.Gradient != nil {
			norm := floats.Norm(loc.Gradient, math.Inf(1))
			if norm < settings.GradientAbsTol {
				return GradientThreshhold
			}
		}
		if loc.F < settings.FunctionAbsTol {
			return FunctionThreshhold
		}
	}

	// Check every step for negative infinity because it could break the
	// linesearches and -inf is the best you can do anyway.
	if math.IsInf(loc.F, -1) {
		return FunctionNegativeInfinity
	}

	if settings.FuncEvaluations > 0 {
		totalFun := stats.FuncEvaluations + stats.FuncGradEvaluations + stats.FuncGradHessEvaluations
		if totalFun >= settings.FuncEvaluations {
			return FunctionEvaluationLimit
		}
	}

	if settings.GradEvaluations > 0 {
		totalGrad := stats.GradEvaluations + stats.FuncGradEvaluations + stats.FuncGradHessEvaluations
		if totalGrad >= settings.GradEvaluations {
			return GradientEvaluationLimit
		}
	}

	if settings.HessEvaluations > 0 {
		totalHess := stats.HessEvaluations + stats.FuncGradHessEvaluations
		if totalHess >= settings.HessEvaluations {
			return HessianEvaluationLimit
		}
	}

	if settings.Runtime > 0 {
		// TODO(vladimir-ch): It would be nice to update Runtime here.
		if stats.Runtime >= settings.Runtime {
			return RuntimeLimit
		}
	}

	if iterType == MajorIteration && settings.MajorIterations > 0 {
		if stats.MajorIterations >= settings.MajorIterations {
			return IterationLimit
		}
	}
	return NotTerminated
}

// invalidate marks unused fields of Location with NaNs. It exists as a help
// for implementers to detect silent bugs in Methods using inconsistent
// Location, e.g., using Gradient after FuncEvaluation request. It is the
// responsibility of Method to make Location valid again.
func invalidate(loc *Location, f, grad, hess bool) {
	if f {
		loc.F = math.NaN()
	}
	if grad && loc.Gradient != nil {
		loc.Gradient[0] = math.NaN()
	}
	if hess && loc.Hessian != nil {
		loc.Hessian.SetSym(0, 0, math.NaN())
	}
}

// evaluate evaluates the function given by funcInfo at xNext, stores the
// answer into loc and updates stats. If loc.X is not equal to xNext, then
// unused fields of loc are set to NaN.
// evaluate panics if the function does not support the requested evalType.
func evaluate(funcInfo *functionInfo, evalType EvaluationType, xNext []float64, loc *Location, stats *Stats) {
	different := !floats.Equal(loc.X, xNext)
	if different {
		copy(loc.X, xNext)
	}
	switch evalType {
	case FuncEvaluation:
		if different {
			invalidate(loc, false, true, true)
		}
		loc.F = funcInfo.function.Func(loc.X)
		stats.FuncEvaluations++
		return
	case GradEvaluation:
		if funcInfo.IsGradient {
			if different {
				invalidate(loc, true, false, true)
			}
			funcInfo.gradient.Grad(loc.X, loc.Gradient)
			stats.GradEvaluations++
			return
		}
		if funcInfo.IsFunctionGradient {
			if different {
				invalidate(loc, false, false, true)
			}
			loc.F = funcInfo.functionGradient.FuncGrad(loc.X, loc.Gradient)
			stats.FuncGradEvaluations++
			return
		}
		if funcInfo.IsFunctionGradientHessian {
			if loc.Hessian == nil {
				loc.Hessian = mat64.NewSymDense(len(loc.X), nil)
			}
			loc.F = funcInfo.functionGradientHessian.FuncGradHess(loc.X, loc.Gradient, loc.Hessian)
			stats.FuncGradHessEvaluations++
			return
		}
	case HessEvaluation:
		if funcInfo.IsHessian {
			if different {
				invalidate(loc, true, true, false)
			}
			funcInfo.hessian.Hess(loc.X, loc.Hessian)
			stats.HessEvaluations++
			return
		}
		if funcInfo.IsFunctionGradientHessian {
			if loc.Gradient == nil {
				loc.Gradient = make([]float64, len(loc.X))
			}
			loc.F = funcInfo.functionGradientHessian.FuncGradHess(loc.X, loc.Gradient, loc.Hessian)
			stats.FuncGradHessEvaluations++
			return
		}
	case FuncGradEvaluation:
		if funcInfo.IsFunctionGradient {
			if different {
				invalidate(loc, false, false, true)
			}
			loc.F = funcInfo.functionGradient.FuncGrad(loc.X, loc.Gradient)
			stats.FuncGradEvaluations++
			return
		}
		if funcInfo.IsGradient {
			if different {
				invalidate(loc, false, false, true)
			}
			loc.F = funcInfo.function.Func(loc.X)
			stats.FuncEvaluations++
			funcInfo.gradient.Grad(loc.X, loc.Gradient)
			stats.GradEvaluations++
			return
		}
		if funcInfo.IsFunctionGradientHessian {
			if loc.Hessian == nil {
				loc.Hessian = mat64.NewSymDense(len(loc.X), nil)
			}
			loc.F = funcInfo.functionGradientHessian.FuncGradHess(loc.X, loc.Gradient, loc.Hessian)
			stats.FuncGradHessEvaluations++
			return
		}
	case FuncGradHessEvaluation:
		if funcInfo.IsFunctionGradientHessian {
			loc.F = funcInfo.functionGradientHessian.FuncGradHess(loc.X, loc.Gradient, loc.Hessian)
			stats.FuncGradHessEvaluations++
			return
		}
		if funcInfo.IsGradient && funcInfo.IsHessian {
			loc.F = funcInfo.function.Func(loc.X)
			stats.FuncEvaluations++
			funcInfo.gradient.Grad(loc.X, loc.Gradient)
			stats.GradEvaluations++
			funcInfo.hessian.Hess(loc.X, loc.Hessian)
			stats.HessEvaluations++
			return
		}
	case NoEvaluation:
		if different {
			// Optimizers should not request NoEvaluation at a new location.
			// The intent and therefore an appropriate action are both unclear.
			panic("optimize: no evaluation requested at new location")
		}
		return
	default:
		panic(fmt.Sprintf("optimize: unknown evaluation type %v", evalType))
	}
	panic(fmt.Sprintf("optimize: objective function does not support %v", evalType))
}

// update updates the stats given the new evaluation
func update(loc *Location, optLoc *Location, stats *Stats, iterType IterationType, startTime time.Time) {
	if iterType == MajorIteration {
		stats.MajorIterations++
	}
	if loc.F <= optLoc.F {
		copyLocation(optLoc, loc)
	}
	stats.Runtime = time.Since(startTime)
}
