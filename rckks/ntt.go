package rckks

import (
	"github.com/ldsec/lattigo/v2/ring"
	"unsafe"
)

// butterfly computes X, Y = U + V*Psi, U - V*Psi mod Q.
func butterfly(U, V, Psi, Q, Qinv uint64) (X, Y uint64) {
	if U > 2*Q {
		U -= 2 * Q
	}
	V = ring.MRedConstant(V, Psi, Q, Qinv)
	X = U + V
	Y = U + 2*Q - V
	return
}

// invbutterfly computes X, Y = U + V, (U - V) * Psi mod Q.
func invbutterfly(U, V, Psi, Q, Qinv uint64) (X, Y uint64) {
	X = U + V
	if X > 2*Q {
		X -= 2 * Q
	}
	Y = ring.MRedConstant(U+2*Q-V, Psi, Q, Qinv) // At the moment it is not possible to use MRedConstant if Q > 61 bits
	return
}

// NTTRCKKS computes the left N half of the NTT in the ring X^{2*N} + 1  of p1 and returns the result on p2.
func NTTRCKKS(r *ring.Ring, p1, p2 *ring.Poly) {
	for x := range r.Modulus {
		nttrckks(p1.Coeffs[x], p2.Coeffs[x], r.N, r.NttPsi[x], r.Modulus[x], r.MredParams[x], r.BredParams[x])
	}
}

// NTTRCKKSLvl computes the left N half of the NTT in the ring X^{2*N} + 1  of p1 and returns the result on p2.
// The value level defines the number of moduli of the input polynomials.
func NTTRCKKSLvl(r *ring.Ring, level uint64, p1, p2 *ring.Poly) {
	for x := uint64(0); x < level+1; x++ {
		nttrckks(p1.Coeffs[x], p2.Coeffs[x], r.N, r.NttPsi[x], r.Modulus[x], r.MredParams[x], r.BredParams[x])
	}
}

// InvNTTRCKKS computes the left N half inverse-NTT in the ring X^{2*N} + 1 of p1 and returns the result on p2.
func InvNTTRCKKS(r *ring.Ring, p1, p2 *ring.Poly) {
	for x := range r.Modulus {
		invnttrckks(p1.Coeffs[x], p2.Coeffs[x], r.N, r.NttPsiInv[x], r.NttNInv[x], r.Modulus[x], r.MredParams[x])
	}
}

// InvNTTRCKKSLvl computes the left N half inverse-NTT in the ring X^{2*N} + 1 of p1 and returns the result on p2.
// The value level defines the number of moduli of the input polynomials.
func InvNTTRCKKSLvl(r *ring.Ring, level uint64, p1, p2 *ring.Poly) {
	for x := uint64(0); x < level+1; x++ {
		invnttrckks(p1.Coeffs[x], p2.Coeffs[x], r.N, r.NttPsiInv[x], r.NttNInv[x], r.Modulus[x], r.MredParams[x])
	}
}


func nttrckks(coeffsIn, coeffsOut []uint64, N uint64, nttPsi []uint64, Q, mredParams uint64, bredParams []uint64) {
	var j1, j2, t uint64
	var F uint64

	// Copy the result of the first round of butterflies on p2 with approximate reduction
	t = N
	F = nttPsi[1]

	for j := uint64(1); j < (N>>1)-7; j = j + 8 {

		xin := (*[8]uint64)(unsafe.Pointer(&coeffsIn[j]))
		yin := (*[8]uint64)(unsafe.Pointer(&coeffsIn[N-j-7]))

		xout := (*[8]uint64)(unsafe.Pointer(&coeffsOut[j]))
		yout := (*[8]uint64)(unsafe.Pointer(&coeffsOut[N-j-7]))

		xout[0], yout[7] = xin[0]+2*Q-ring.MRedConstant(yin[7], F, Q, mredParams), yin[7]+2*Q-ring.MRedConstant(xin[0], F, Q, mredParams)
		xout[1], yout[6] = xin[1]+2*Q-ring.MRedConstant(yin[6], F, Q, mredParams), yin[6]+2*Q-ring.MRedConstant(xin[1], F, Q, mredParams)
		xout[2], yout[5] = xin[2]+2*Q-ring.MRedConstant(yin[5], F, Q, mredParams), yin[5]+2*Q-ring.MRedConstant(xin[2], F, Q, mredParams)
		xout[3], yout[4] = xin[3]+2*Q-ring.MRedConstant(yin[4], F, Q, mredParams), yin[4]+2*Q-ring.MRedConstant(xin[3], F, Q, mredParams)
		xout[4], yout[3] = xin[4]+2*Q-ring.MRedConstant(yin[3], F, Q, mredParams), yin[3]+2*Q-ring.MRedConstant(xin[4], F, Q, mredParams)
		xout[5], yout[2] = xin[5]+2*Q-ring.MRedConstant(yin[2], F, Q, mredParams), yin[2]+2*Q-ring.MRedConstant(xin[5], F, Q, mredParams)
		xout[6], yout[1] = xin[6]+2*Q-ring.MRedConstant(yin[1], F, Q, mredParams), yin[1]+2*Q-ring.MRedConstant(xin[6], F, Q, mredParams)
		xout[7], yout[0] = xin[7]+2*Q-ring.MRedConstant(yin[0], F, Q, mredParams), yin[0]+2*Q-ring.MRedConstant(xin[7], F, Q, mredParams)
	}

	j := (N >> 1) - 7
	xin := (*[7]uint64)(unsafe.Pointer(&coeffsIn[j]))
	yin := (*[7]uint64)(unsafe.Pointer(&coeffsIn[N-j-6]))
	xout := (*[7]uint64)(unsafe.Pointer(&coeffsOut[j]))
	yout := (*[7]uint64)(unsafe.Pointer(&coeffsOut[N-j-6]))

	xout[0], yout[6] = xin[0]+2*Q-ring.MRedConstant(yin[6], F, Q, mredParams), yin[6]+2*Q-ring.MRedConstant(xin[0], F, Q, mredParams)
	xout[1], yout[5] = xin[1]+2*Q-ring.MRedConstant(yin[5], F, Q, mredParams), yin[5]+2*Q-ring.MRedConstant(xin[1], F, Q, mredParams)
	xout[2], yout[4] = xin[2]+2*Q-ring.MRedConstant(yin[4], F, Q, mredParams), yin[4]+2*Q-ring.MRedConstant(xin[2], F, Q, mredParams)
	xout[3], yout[3] = xin[3]+2*Q-ring.MRedConstant(yin[3], F, Q, mredParams), yin[3]+2*Q-ring.MRedConstant(xin[3], F, Q, mredParams)
	xout[4], yout[2] = xin[4]+2*Q-ring.MRedConstant(yin[2], F, Q, mredParams), yin[2]+2*Q-ring.MRedConstant(xin[4], F, Q, mredParams)
	xout[5], yout[1] = xin[5]+2*Q-ring.MRedConstant(yin[1], F, Q, mredParams), yin[1]+2*Q-ring.MRedConstant(xin[5], F, Q, mredParams)
	xout[6], yout[0] = xin[6]+2*Q-ring.MRedConstant(yin[0], F, Q, mredParams), yin[0]+2*Q-ring.MRedConstant(xin[6], F, Q, mredParams)

	coeffsOut[N>>1] = coeffsIn[N>>1] + 2*Q - ring.MRedConstant(coeffsIn[N>>1], F, Q, mredParams)

	// Continue the rest of the second to the n-1 butterflies on p2 with approximate reduction
	for m := uint64(2); m < 2*N; m <<= 1 {

		t >>= 1

		if t >= 8 {

			for i := uint64(0); i < m>>1; i++ {

				j1 = (i * t) << 1

				j2 = j1 + t - 1

				F = nttPsi[m+i]

				for j := j1; j <= j2; j = j + 8 {

					x := (*[8]uint64)(unsafe.Pointer(&coeffsOut[j]))
					y := (*[8]uint64)(unsafe.Pointer(&coeffsOut[j+t]))

					x[0], y[0] = butterfly(x[0], y[0], F, Q, mredParams)
					x[1], y[1] = butterfly(x[1], y[1], F, Q, mredParams)
					x[2], y[2] = butterfly(x[2], y[2], F, Q, mredParams)
					x[3], y[3] = butterfly(x[3], y[3], F, Q, mredParams)
					x[4], y[4] = butterfly(x[4], y[4], F, Q, mredParams)
					x[5], y[5] = butterfly(x[5], y[5], F, Q, mredParams)
					x[6], y[6] = butterfly(x[6], y[6], F, Q, mredParams)
					x[7], y[7] = butterfly(x[7], y[7], F, Q, mredParams)
				}
			}

		} else if t == 4 {

			for i := uint64(0); i < m>>1; i = i + 2 {

				j1 = (i * t) << 1

				psi := (*[2]uint64)(unsafe.Pointer(&nttPsi[m+i]))
				x := (*[16]uint64)(unsafe.Pointer(&coeffsOut[j1]))

				x[0], x[4] = butterfly(x[0], x[4], psi[0], Q, mredParams)
				x[1], x[5] = butterfly(x[1], x[5], psi[0], Q, mredParams)
				x[2], x[6] = butterfly(x[2], x[6], psi[0], Q, mredParams)
				x[3], x[7] = butterfly(x[3], x[7], psi[0], Q, mredParams)
				x[8], x[12] = butterfly(x[8], x[12], psi[1], Q, mredParams)
				x[9], x[13] = butterfly(x[9], x[13], psi[1], Q, mredParams)
				x[10], x[14] = butterfly(x[10], x[14], psi[1], Q, mredParams)
				x[11], x[15] = butterfly(x[11], x[15], psi[1], Q, mredParams)

			}

		} else if t == 2 {

			for i := uint64(0); i < m>>1; i = i + 4 {

				j1 = (i * t) << 1

				psi := (*[4]uint64)(unsafe.Pointer(&nttPsi[m+i]))
				x := (*[16]uint64)(unsafe.Pointer(&coeffsOut[j1]))

				x[0], x[2] = butterfly(x[0], x[2], psi[0], Q, mredParams)
				x[1], x[3] = butterfly(x[1], x[3], psi[0], Q, mredParams)
				x[4], x[6] = butterfly(x[4], x[6], psi[1], Q, mredParams)
				x[5], x[7] = butterfly(x[5], x[7], psi[1], Q, mredParams)
				x[8], x[10] = butterfly(x[8], x[10], psi[2], Q, mredParams)
				x[9], x[11] = butterfly(x[9], x[11], psi[2], Q, mredParams)
				x[12], x[14] = butterfly(x[12], x[14], psi[3], Q, mredParams)
				x[13], x[15] = butterfly(x[13], x[15], psi[3], Q, mredParams)
			}

		} else {

			for i := uint64(0); i < m>>1; i = i + 8 {

				psi := (*[8]uint64)(unsafe.Pointer(&nttPsi[m+i]))
				x := (*[16]uint64)(unsafe.Pointer(&coeffsOut[2*i]))

				x[0], x[1] = butterfly(x[0], x[1], psi[0], Q, mredParams)
				x[2], x[3] = butterfly(x[2], x[3], psi[1], Q, mredParams)
				x[4], x[5] = butterfly(x[4], x[5], psi[2], Q, mredParams)
				x[6], x[7] = butterfly(x[6], x[7], psi[3], Q, mredParams)
				x[8], x[9] = butterfly(x[8], x[9], psi[4], Q, mredParams)
				x[10], x[11] = butterfly(x[10], x[11], psi[5], Q, mredParams)
				x[12], x[13] = butterfly(x[12], x[13], psi[6], Q, mredParams)
				x[14], x[15] = butterfly(x[14], x[15], psi[7], Q, mredParams)
			}
		}
	}

	// Finish with an exact reduction
	for i := uint64(0); i < N; i = i + 8 {

		x := (*[8]uint64)(unsafe.Pointer(&coeffsOut[i]))

		x[0] = ring.BRedAdd(x[0], Q, bredParams)
		x[1] = ring.BRedAdd(x[1], Q, bredParams)
		x[2] = ring.BRedAdd(x[2], Q, bredParams)
		x[3] = ring.BRedAdd(x[3], Q, bredParams)
		x[4] = ring.BRedAdd(x[4], Q, bredParams)
		x[5] = ring.BRedAdd(x[5], Q, bredParams)
		x[6] = ring.BRedAdd(x[6], Q, bredParams)
		x[7] = ring.BRedAdd(x[7], Q, bredParams)
	}
}


func invnttrckks(coeffsIn, coeffsOut []uint64, N uint64, nttPsiInv []uint64, nttNInv, Q, mredParams uint64) {

	var j1, j2, h, t uint64
	var F uint64

	// Copy the result of the first round of butterflies on p2 with approximate reduction
	t = 1
	h = N

	for i := uint64(0); i < h>>1; i = i + 8 {

		psi := (*[8]uint64)(unsafe.Pointer(&nttPsiInv[h+i]))
		xin := (*[16]uint64)(unsafe.Pointer(&coeffsIn[2*i]))
		xout := (*[16]uint64)(unsafe.Pointer(&coeffsOut[2*i]))

		xout[0], xout[1] = invbutterfly(xin[0], xin[1], psi[0], Q, mredParams)
		xout[2], xout[3] = invbutterfly(xin[2], xin[3], psi[1], Q, mredParams)
		xout[4], xout[5] = invbutterfly(xin[4], xin[5], psi[2], Q, mredParams)
		xout[6], xout[7] = invbutterfly(xin[6], xin[7], psi[3], Q, mredParams)
		xout[8], xout[9] = invbutterfly(xin[8], xin[9], psi[4], Q, mredParams)
		xout[10], xout[11] = invbutterfly(xin[10], xin[11], psi[5], Q, mredParams)
		xout[12], xout[13] = invbutterfly(xin[12], xin[13], psi[6], Q, mredParams)
		xout[14], xout[15] = invbutterfly(xin[14], xin[15], psi[7], Q, mredParams)
	}

	// Continue the rest of the second to the n-1 butterflies on p2 with approximate reduction
	t <<= 1
	for m := N; m > 2; m >>= 1 {

		j1 = 0
		h = m >> 1

		if t >= 8 {

			for i := uint64(0); i < h>>1; i++ {

				j2 = j1 + t - 1

				F = nttPsiInv[h+i]

				for j := j1; j <= j2; j = j + 8 {

					x := (*[8]uint64)(unsafe.Pointer(&coeffsOut[j]))
					y := (*[8]uint64)(unsafe.Pointer(&coeffsOut[j+t]))

					x[0], y[0] = invbutterfly(x[0], y[0], F, Q, mredParams)
					x[1], y[1] = invbutterfly(x[1], y[1], F, Q, mredParams)
					x[2], y[2] = invbutterfly(x[2], y[2], F, Q, mredParams)
					x[3], y[3] = invbutterfly(x[3], y[3], F, Q, mredParams)
					x[4], y[4] = invbutterfly(x[4], y[4], F, Q, mredParams)
					x[5], y[5] = invbutterfly(x[5], y[5], F, Q, mredParams)
					x[6], y[6] = invbutterfly(x[6], y[6], F, Q, mredParams)
					x[7], y[7] = invbutterfly(x[7], y[7], F, Q, mredParams)
				}

				j1 = j1 + (t << 1)
			}

		} else if t == 4 {

			for i := uint64(0); i < h>>1; i = i + 2 {

				psi := (*[2]uint64)(unsafe.Pointer(&nttPsiInv[h+i]))
				x := (*[16]uint64)(unsafe.Pointer(&coeffsOut[j1]))

				x[0], x[4] = invbutterfly(x[0], x[4], psi[0], Q, mredParams)
				x[1], x[5] = invbutterfly(x[1], x[5], psi[0], Q, mredParams)
				x[2], x[6] = invbutterfly(x[2], x[6], psi[0], Q, mredParams)
				x[3], x[7] = invbutterfly(x[3], x[7], psi[0], Q, mredParams)
				x[8], x[12] = invbutterfly(x[8], x[12], psi[1], Q, mredParams)
				x[9], x[13] = invbutterfly(x[9], x[13], psi[1], Q, mredParams)
				x[10], x[14] = invbutterfly(x[10], x[14], psi[1], Q, mredParams)
				x[11], x[15] = invbutterfly(x[11], x[15], psi[1], Q, mredParams)

				j1 = j1 + (t << 2)
			}

		} else {

			for i := uint64(0); i < h>>1; i = i + 4 {

				psi := (*[4]uint64)(unsafe.Pointer(&nttPsiInv[h+i]))
				x := (*[16]uint64)(unsafe.Pointer(&coeffsOut[j1]))

				x[0], x[2] = invbutterfly(x[0], x[2], psi[0], Q, mredParams)
				x[1], x[3] = invbutterfly(x[1], x[3], psi[0], Q, mredParams)
				x[4], x[6] = invbutterfly(x[4], x[6], psi[1], Q, mredParams)
				x[5], x[7] = invbutterfly(x[5], x[7], psi[1], Q, mredParams)
				x[8], x[10] = invbutterfly(x[8], x[10], psi[2], Q, mredParams)
				x[9], x[11] = invbutterfly(x[9], x[11], psi[2], Q, mredParams)
				x[12], x[14] = invbutterfly(x[12], x[14], psi[3], Q, mredParams)
				x[13], x[15] = invbutterfly(x[13], x[15], psi[3], Q, mredParams)

				j1 = j1 + (t << 3)
			}
		}

		t <<= 1
	}

	F = nttPsiInv[1]

	for j := uint64(1); j < (N>>1)-7; j = j + 8 {

		xout := (*[8]uint64)(unsafe.Pointer(&coeffsOut[j]))
		yout := (*[8]uint64)(unsafe.Pointer(&coeffsOut[N-j-7]))

		xout[0], yout[7] = xout[0]+2*Q-ring.MRedConstant(yout[7], F, Q, mredParams), yout[7]+2*Q-ring.MRedConstant(xout[0], F, Q, mredParams)
		xout[1], yout[6] = xout[1]+2*Q-ring.MRedConstant(yout[6], F, Q, mredParams), yout[6]+2*Q-ring.MRedConstant(xout[1], F, Q, mredParams)
		xout[2], yout[5] = xout[2]+2*Q-ring.MRedConstant(yout[5], F, Q, mredParams), yout[5]+2*Q-ring.MRedConstant(xout[2], F, Q, mredParams)
		xout[3], yout[4] = xout[3]+2*Q-ring.MRedConstant(yout[4], F, Q, mredParams), yout[4]+2*Q-ring.MRedConstant(xout[3], F, Q, mredParams)
		xout[4], yout[3] = xout[4]+2*Q-ring.MRedConstant(yout[3], F, Q, mredParams), yout[3]+2*Q-ring.MRedConstant(xout[4], F, Q, mredParams)
		xout[5], yout[2] = xout[5]+2*Q-ring.MRedConstant(yout[2], F, Q, mredParams), yout[2]+2*Q-ring.MRedConstant(xout[5], F, Q, mredParams)
		xout[6], yout[1] = xout[6]+2*Q-ring.MRedConstant(yout[1], F, Q, mredParams), yout[1]+2*Q-ring.MRedConstant(xout[6], F, Q, mredParams)
		xout[7], yout[0] = xout[7]+2*Q-ring.MRedConstant(yout[0], F, Q, mredParams), yout[0]+2*Q-ring.MRedConstant(xout[7], F, Q, mredParams)
	}

	j := (N >> 1) - 7
	xout := (*[7]uint64)(unsafe.Pointer(&coeffsOut[j]))
	yout := (*[7]uint64)(unsafe.Pointer(&coeffsOut[N-j-6]))

	xout[0], yout[6] = xout[0]+2*Q-ring.MRedConstant(yout[6], F, Q, mredParams), yout[6]+2*Q-ring.MRedConstant(xout[0], F, Q, mredParams)
	xout[1], yout[5] = xout[1]+2*Q-ring.MRedConstant(yout[5], F, Q, mredParams), yout[5]+2*Q-ring.MRedConstant(xout[1], F, Q, mredParams)
	xout[2], yout[4] = xout[2]+2*Q-ring.MRedConstant(yout[4], F, Q, mredParams), yout[4]+2*Q-ring.MRedConstant(xout[2], F, Q, mredParams)
	xout[3], yout[3] = xout[3]+2*Q-ring.MRedConstant(yout[3], F, Q, mredParams), yout[3]+2*Q-ring.MRedConstant(xout[3], F, Q, mredParams)
	xout[4], yout[2] = xout[4]+2*Q-ring.MRedConstant(yout[2], F, Q, mredParams), yout[2]+2*Q-ring.MRedConstant(xout[4], F, Q, mredParams)
	xout[5], yout[1] = xout[5]+2*Q-ring.MRedConstant(yout[1], F, Q, mredParams), yout[1]+2*Q-ring.MRedConstant(xout[5], F, Q, mredParams)
	xout[6], yout[0] = xout[6]+2*Q-ring.MRedConstant(yout[0], F, Q, mredParams), yout[0]+2*Q-ring.MRedConstant(xout[6], F, Q, mredParams)

	coeffsOut[N>>1] = coeffsOut[N>>1] + 2*Q - ring.MRedConstant(coeffsOut[N>>1], F, Q, mredParams)

	// Finish with an exact reduction
	for i := uint64(0); i < N; i = i + 8 {

		x := (*[8]uint64)(unsafe.Pointer(&coeffsOut[i]))

		x[0] = ring.MRed(x[0], nttNInv, Q, mredParams)
		x[1] = ring.MRed(x[1], nttNInv, Q, mredParams)
		x[2] = ring.MRed(x[2], nttNInv, Q, mredParams)
		x[3] = ring.MRed(x[3], nttNInv, Q, mredParams)
		x[4] = ring.MRed(x[4], nttNInv, Q, mredParams)
		x[5] = ring.MRed(x[5], nttNInv, Q, mredParams)
		x[6] = ring.MRed(x[6], nttNInv, Q, mredParams)
		x[7] = ring.MRed(x[7], nttNInv, Q, mredParams)
	}

	coeffsOut[0] = ring.CRed(2*coeffsOut[0], Q)
}