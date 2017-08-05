// Copyright ©2016 The gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//+build !amd64 noasm appengine

package c128

// AxpyUnitary is
//  for i, v := range x {
//  	y[i] += alpha * v
//  }
func AxpyUnitary(alpha complex128, x, y []complex128) {
	for i, v := range x {
		y[i] += alpha * v
	}
}

// AxpyUnitaryTo is
//  for i, v := range x {
//  	dst[i] = alpha*v + y[i]
//  }
func AxpyUnitaryTo(dst []complex128, alpha complex128, x, y []complex128) {
	for i, v := range x {
		dst[i] = alpha*v + y[i]
	}
}

// AxpyInc is
//  for i := 0; i < int(n); i++ {
//  	y[iy] += alpha * x[ix]
//  	ix += incX
//  	iy += incY
//  }
func AxpyInc(alpha complex128, x, y []complex128, n, incX, incY, ix, iy uintptr) {
	for i := 0; i < int(n); i++ {
		y[iy] += alpha * x[ix]
		ix += incX
		iy += incY
	}
}

// AxpyIncTo is
//  for i := 0; i < int(n); i++ {
//  	dst[idst] = alpha*x[ix] + y[iy]
//  	ix += incX
//  	iy += incY
//  	idst += incDst
//  }
func AxpyIncTo(dst []complex128, incDst, idst uintptr, alpha complex128, x, y []complex128, n, incX, incY, ix, iy uintptr) {
	for i := 0; i < int(n); i++ {
		dst[idst] = alpha*x[ix] + y[iy]
		ix += incX
		iy += incY
		idst += incDst
	}
}

// DscalUnitary is
//  for i, v := range x {
//  	x[i] = complex(real(v)*alpha, imag(v))
//  }
func DscalUnitary(alpha float64, x []complex128) {
	for i, v := range x {
		x[i] = complex(real(v)*alpha, imag(v))
	}
}

// DscalInc is
//  for i := 0; i < n; i++ {
//  	x[i*inc] = complex(real(x[i*inc])*alpha, imag(x[i*inc]))
//  }
func DscalInc(n int, alpha float64, x []complex128, inc int) {
	for i := 0; i < n; i++ {
		x[i*inc] = complex(real(x[i*inc])*alpha, imag(x[i*inc]))
	}
}

// ScalUnitary is
//  for i := range x {
//  	x[i] *= alpha
//  }
func ScalUnitary(alpha complex128, x []complex128) {
	for i := range x {
		x[i] *= alpha
	}
}
