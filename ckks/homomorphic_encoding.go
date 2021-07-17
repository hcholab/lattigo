package ckks

import (
	"github.com/ldsec/lattigo/v2/utils"
	"math"
)

// EncodingMatrices is a struct storing the factorized DFT matrix
type EncodingMatrices struct {
	Matrices []PtDiagMatrix
}

// EncodingMatricesParameters is a struct storing the parameters to generate the factorized DFT matrix
type EncodingMatricesParameters struct {
	Forward       bool // True : CoeffsToSlots (Encoding) - False : SlotsToCoeffs (Decoding)
	LevelStart    int
	BitReversed   bool    // Flag for bit-reverseed input to the DFT (with bit-reversed output), by default false.
	BSGSRatio     float64 // n1/n2 ratio for the bsgs algo for matrix x vector eval
	ScalingFactor [][]float64
}

// Depth returns the number of levels allocated.
// If actual == true then returns the number of moduli consumed, else
// returns the factorization depth.
func (mParams *EncodingMatricesParameters) Depth(actual bool) (depth int) {
	if actual {
		depth = len(mParams.ScalingFactor)
	} else {
		for i := range mParams.ScalingFactor {
			for range mParams.ScalingFactor[i] {
				depth++
			}
		}
	}
	return
}

// Levels returns the index of the Qi used int CoeffsToSlots
func (mParams *EncodingMatricesParameters) Levels() (levels []int) {
	levels = []int{}
	trueDepth := mParams.Depth(true)
	for i := range mParams.ScalingFactor {
		for range mParams.ScalingFactor[trueDepth-1-i] {
			levels = append(levels, mParams.LevelStart-i)
		}
	}

	return
}

// Rotations returns the list of rotations performed during the CoeffsToSlot operation.
func (mParams *EncodingMatricesParameters) Rotations(logN, logSlots int) (rotations []int) {
	rotations = []int{}

	slots := 1 << logSlots
	dslots := slots
	if logSlots < logN-1 {
		dslots <<= 1
		if mParams.Forward {
			rotations = append(rotations, slots)
		}
	}

	indexCtS := computeBootstrappingDFTIndexMap(logN, logSlots, mParams.Depth(false), mParams.Forward, mParams.BitReversed)

	// Coeffs to Slots rotations
	for i, pVec := range indexCtS {
		N1 := findbestbabygiantstepsplit(pVec, dslots, mParams.BSGSRatio)
		rotations = addMatrixRotToList(pVec, rotations, N1, slots, !mParams.Forward && logSlots < logN-1 && i == 0)
	}

	return
}

// GenCoeffsToSlotsMatrix generates the factorized encoding matrix
// scaling : constant by witch the all the matrices will be multuplied by
// encoder : ckks.Encoder
func (encoder *encoderComplex128) GenHomomorphicEncodingMatrices(mParams EncodingMatricesParameters, scaling complex128) EncodingMatrices {
	logSlots := encoder.params.LogSlots()

	slots := 1 << logSlots
	depth := mParams.Depth(false)
	logdSlots := logSlots + 1
	if logdSlots == encoder.params.LogN() {
		logdSlots--
	}

	roots := computeRoots(slots << 1)
	pow5 := make([]int, (slots<<1)+1)
	pow5[0] = 1
	for i := 1; i < (slots<<1)+1; i++ {
		pow5[i] = pow5[i-1] * 5
		pow5[i] &= (slots << 2) - 1
	}

	ctsLevels := mParams.Levels()

	// CoeffsToSlots vectors
	matrices := make([]PtDiagMatrix, len(ctsLevels))
	pVecDFT := computeDFTMatrices(logSlots, logdSlots, depth, roots, pow5, scaling, mParams.Forward, mParams.BitReversed)
	cnt := 0
	trueDepth := mParams.Depth(true)
	for i := range mParams.ScalingFactor {
		for j := range mParams.ScalingFactor[trueDepth-i-1] {
			matrices[cnt] = encoder.EncodeDiagMatrixBSGSAtLvl(ctsLevels[cnt], pVecDFT[cnt], mParams.ScalingFactor[trueDepth-i-1][j], mParams.BSGSRatio, logdSlots)
			cnt++
		}
	}

	return EncodingMatrices{Matrices: matrices}
}

// CoeffsToSlots applies the homomorphic encoding
func (eval *evaluator) CoeffsToSlots(ctIn *Ciphertext, ctsMatrices EncodingMatrices) (ctReal, ctImag *Ciphertext) {

	var zV, zVconj *Ciphertext

	zV = eval.dft(ctIn, ctsMatrices.Matrices)

	zVconj = eval.ConjugateNew(zV)

	// The real part is stored in ct0
	ctReal = eval.AddNew(zV, zVconj)

	// The imaginary part is stored in ct1
	ctImag = eval.SubNew(zV, zVconj)

	eval.DivByi(ctImag, ctImag)

	// If repacking, then ct0 and ct1 right n/2 slots are zero.
	if eval.params.LogSlots() < eval.params.LogN()-1 {
		eval.Rotate(ctImag, eval.params.Slots(), ctImag)
		eval.Add(ctReal, ctImag, ctReal)
		return ctReal, nil
	}

	zV = nil
	zVconj = nil

	return ctReal, ctImag
}

// SlotsToCoeffs applies the homomorphic decoding
func (eval *evaluator) SlotsToCoeffs(ctReal, ctImag *Ciphertext, stcMatrices EncodingMatrices) (ctOut *Ciphertext) {

	// If full packing, the repacking can be done directly using ct0 and ct1.
	if ctImag != nil {
		eval.MultByi(ctImag, ctImag)
		eval.Add(ctReal, ctImag, ctReal)
	}

	ctImag = nil

	return eval.dft(ctReal, stcMatrices.Matrices)
}

func (eval *evaluator) dft(vec *Ciphertext, plainVectors []PtDiagMatrix) *Ciphertext {

	// Sequentially multiplies w with the provided dft matrices.
	for _, plainVector := range plainVectors {
		scale := vec.Scale
		vec = eval.LinearTransform(vec, plainVector)[0]
		if err := eval.Rescale(vec, scale, vec); err != nil {
			panic(err)
		}
	}

	return vec
}

func computeRoots(N int) (roots []complex128) {

	var angle float64

	m := N << 1

	roots = make([]complex128, m)

	roots[0] = 1

	for i := 1; i < m; i++ {
		angle = 6.283185307179586 * float64(i) / float64(m)
		roots[i] = complex(math.Cos(angle), math.Sin(angle))
	}

	return
}

func fftPlainVec(logN, dslots int, roots []complex128, pow5 []int) (a, b, c [][]complex128) {

	var N, m, index, tt, gap, k, mask, idx1, idx2 int

	N = 1 << logN

	a = make([][]complex128, logN)
	b = make([][]complex128, logN)
	c = make([][]complex128, logN)

	var size int
	if 2*N == dslots {
		size = 2
	} else {
		size = 1
	}

	index = 0
	for m = 2; m <= N; m <<= 1 {

		a[index] = make([]complex128, dslots)
		b[index] = make([]complex128, dslots)
		c[index] = make([]complex128, dslots)

		tt = m >> 1

		for i := 0; i < N; i += m {

			gap = N / m
			mask = (m << 2) - 1

			for j := 0; j < m>>1; j++ {

				k = (pow5[j] & mask) * gap

				idx1 = i + j
				idx2 = i + j + tt

				for u := 0; u < size; u++ {
					a[index][idx1+u*N] = 1
					a[index][idx2+u*N] = -roots[k]
					b[index][idx1+u*N] = roots[k]
					c[index][idx2+u*N] = 1
				}
			}
		}

		index++
	}

	return
}

func fftInvPlainVec(logN, dslots int, roots []complex128, pow5 []int) (a, b, c [][]complex128) {

	var N, m, index, tt, gap, k, mask, idx1, idx2 int

	N = 1 << logN

	a = make([][]complex128, logN)
	b = make([][]complex128, logN)
	c = make([][]complex128, logN)

	var size int
	if 2*N == dslots {
		size = 2
	} else {
		size = 1
	}

	index = 0
	for m = N; m >= 2; m >>= 1 {

		a[index] = make([]complex128, dslots)
		b[index] = make([]complex128, dslots)
		c[index] = make([]complex128, dslots)

		tt = m >> 1

		for i := 0; i < N; i += m {

			gap = N / m
			mask = (m << 2) - 1

			for j := 0; j < m>>1; j++ {

				k = ((m << 2) - (pow5[j] & mask)) * gap

				idx1 = i + j
				idx2 = i + j + tt

				for u := 0; u < size; u++ {

					a[index][idx1+u*N] = 1
					a[index][idx2+u*N] = -roots[k]
					b[index][idx1+u*N] = 1
					c[index][idx2+u*N] = roots[k]
				}
			}
		}

		index++
	}

	return
}

func addMatrixRotToList(pVec map[int]bool, rotations []int, N1, slots int, repack bool) []int {

	if len(pVec) < 3 {
		for j := range pVec {
			if !utils.IsInSliceInt(j, rotations) {
				rotations = append(rotations, j)
			}
		}
	} else {
		var index int
		for j := range pVec {

			index = (j / N1) * N1

			if repack {
				// Sparse repacking, occurring during the first DFT matrix of the CoeffsToSlots.
				index &= (2*slots - 1)
			} else {
				// Other cases
				index &= (slots - 1)
			}

			if index != 0 && !utils.IsInSliceInt(index, rotations) {
				rotations = append(rotations, index)
			}

			index = j & (N1 - 1)

			if index != 0 && !utils.IsInSliceInt(index, rotations) {
				rotations = append(rotations, index)
			}
		}
	}

	return rotations
}

func computeBootstrappingDFTIndexMap(logN, logSlots, maxDepth int, forward, bitreversed bool) (rotationMap []map[int]bool) {

	var level, depth, nextLevel int

	level = logSlots

	rotationMap = make([]map[int]bool, maxDepth)

	// We compute the chain of merge in order or reverse order depending if its DFT or InvDFT because
	// the way the levels are collapsed has an impact on the total number of rotations and keys to be
	// stored. Ex. instead of using 255 + 64 plaintext vectors, we can use 127 + 128 plaintext vectors
	// by reversing the order of the merging.
	merge := make([]int, maxDepth)
	for i := 0; i < maxDepth; i++ {

		depth = int(math.Ceil(float64(level) / float64(maxDepth-i)))

		if forward {
			merge[i] = depth
		} else {
			merge[len(merge)-i-1] = depth

		}

		level -= depth
	}

	level = logSlots
	for i := 0; i < maxDepth; i++ {

		if logSlots < logN-1 && !forward && i == 0 {

			// Special initial matrix for the repacking before SlotsToCoeffs
			rotationMap[i] = genWfftRepackIndexMap(logSlots, level)

			// Merges this special initial matrix with the first layer of SlotsToCoeffs DFT
			rotationMap[i] = nextLevelfftIndexMap(rotationMap[i], logSlots, 2<<logSlots, level, forward, bitreversed)

			// Continues the merging with the next layers if the total depth requires it.
			nextLevel = level - 1
			for j := 0; j < merge[i]-1; j++ {
				rotationMap[i] = nextLevelfftIndexMap(rotationMap[i], logSlots, 2<<logSlots, nextLevel, forward, bitreversed)
				nextLevel--
			}

		} else {
			// First layer of the i-th level of the DFT
			rotationMap[i] = genWfftIndexMap(logSlots, level, forward, bitreversed)

			// Merges the layer with the next levels of the DFT if the total depth requires it.
			nextLevel = level - 1
			for j := 0; j < merge[i]-1; j++ {
				rotationMap[i] = nextLevelfftIndexMap(rotationMap[i], logSlots, 1<<logSlots, nextLevel, forward, bitreversed)
				nextLevel--
			}
		}

		level -= merge[i]
	}

	return
}

func genWfftIndexMap(logL, level int, forward, bitreversed bool) (vectors map[int]bool) {

	var rot int

	if forward && !bitreversed || !forward && bitreversed {
		rot = 1 << (level - 1)
	} else {
		rot = 1 << (logL - level)
	}

	vectors = make(map[int]bool)
	vectors[0] = true
	vectors[rot] = true
	vectors[(1<<logL)-rot] = true
	return
}

func genWfftRepackIndexMap(logL, level int) (vectors map[int]bool) {
	vectors = make(map[int]bool)
	vectors[0] = true
	vectors[(1 << logL)] = true
	return
}

func nextLevelfftIndexMap(vec map[int]bool, logL, N, nextLevel int, forward, bitreversed bool) (newVec map[int]bool) {

	var rot int

	newVec = make(map[int]bool)

	if forward && !bitreversed || !forward && bitreversed {
		rot = (1 << (nextLevel - 1)) & (N - 1)
	} else {
		rot = (1 << (logL - nextLevel)) & (N - 1)
	}

	for i := range vec {
		newVec[i] = true
		newVec[(i+rot)&(N-1)] = true
		newVec[(i-rot)&(N-1)] = true
	}

	return
}

func computeDFTMatrices(logSlots, logdSlots, maxDepth int, roots []complex128, pow5 []int, diffscale complex128, inverse, bitreversed bool) (plainVector []map[int][]complex128) {

	var fftLevel, depth, nextfftLevel int

	fftLevel = logSlots

	var a, b, c [][]complex128

	if inverse {
		a, b, c = fftInvPlainVec(logSlots, 1<<logdSlots, roots, pow5)
	} else {
		a, b, c = fftPlainVec(logSlots, 1<<logdSlots, roots, pow5)
	}

	plainVector = make([]map[int][]complex128, maxDepth)

	// We compute the chain of merge in order or reverse order depending if its DFT or InvDFT because
	// the way the levels are collapsed has an inpact on the total number of rotations and keys to be
	// stored. Ex. instead of using 255 + 64 plaintext vectors, we can use 127 + 128 plaintext vectors
	// by reversing the order of the merging.
	merge := make([]int, maxDepth)
	for i := 0; i < maxDepth; i++ {

		depth = int(math.Ceil(float64(fftLevel) / float64(maxDepth-i)))

		if inverse {
			merge[i] = depth
		} else {
			merge[len(merge)-i-1] = depth

		}

		fftLevel -= depth
	}

	fftLevel = logSlots
	for i := 0; i < maxDepth; i++ {

		if logSlots != logdSlots && !inverse && i == 0 {

			// Special initial matrix for the repacking before SlotsToCoeffs
			plainVector[i] = genRepackMatrix(logSlots, bitreversed)

			// Merges this special initial matrix with the first layer of SlotsToCoeffs DFT
			plainVector[i] = multiplyFFTMatrixWithNextFFTLevel(plainVector[i], logSlots, 2<<logSlots, fftLevel, a[logSlots-fftLevel], b[logSlots-fftLevel], c[logSlots-fftLevel], inverse, bitreversed)

			// Continues the merging with the next layers if the total depth requires it.
			nextfftLevel = fftLevel - 1
			for j := 0; j < merge[i]-1; j++ {
				plainVector[i] = multiplyFFTMatrixWithNextFFTLevel(plainVector[i], logSlots, 2<<logSlots, nextfftLevel, a[logSlots-nextfftLevel], b[logSlots-nextfftLevel], c[logSlots-nextfftLevel], inverse, bitreversed)
				nextfftLevel--
			}

		} else {
			// First layer of the i-th level of the DFT
			plainVector[i] = genFFTDiagMatrix(logSlots, fftLevel, a[logSlots-fftLevel], b[logSlots-fftLevel], c[logSlots-fftLevel], inverse, bitreversed)

			// Merges the layer with the next levels of the DFT if the total depth requires it.
			nextfftLevel = fftLevel - 1
			for j := 0; j < merge[i]-1; j++ {
				plainVector[i] = multiplyFFTMatrixWithNextFFTLevel(plainVector[i], logSlots, 1<<logSlots, nextfftLevel, a[logSlots-nextfftLevel], b[logSlots-nextfftLevel], c[logSlots-nextfftLevel], inverse, bitreversed)
				nextfftLevel--
			}
		}

		fftLevel -= merge[i]
	}

	// Repacking after the CoeffsToSlots (we multiply the last DFT matrix with the vector [1, 1, ..., 1, 1, 0, 0, ..., 0, 0]).
	if logSlots != logdSlots && inverse {
		for j := range plainVector[maxDepth-1] {
			for x := 0; x < 1<<logSlots; x++ {
				plainVector[maxDepth-1][j][x+(1<<logSlots)] = complex(0, 0)
			}
		}
	}

	// Rescaling of the DFT matrix of the SlotsToCoeffs/CoeffsToSlots
	for j := range plainVector {
		for x := range plainVector[j] {
			for i := range plainVector[j][x] {
				plainVector[j][x][i] *= diffscale
			}
		}
	}

	/*
		for j := range plainVector {
			for x := 0; x < 1<<logSlots; x++{
				if plainVector[j][x] != nil{
					fmt.Printf("%d :", x)
					for i := range plainVector[j][x] {
						fmt.Printf("%0.4f", plainVector[j][x][i])
					}
					fmt.Printf("\n")
				}

			}
			fmt.Printf("\n")
		}
	*/

	return
}

func genFFTDiagMatrix(logL, fftLevel int, a, b, c []complex128, forward, bitreversed bool) (vectors map[int][]complex128) {

	var rot int

	if forward && !bitreversed || !forward && bitreversed {
		rot = 1 << (fftLevel - 1)
	} else {
		rot = 1 << (logL - fftLevel)
	}

	vectors = make(map[int][]complex128)

	if bitreversed {
		SliceBitReverseInPlaceComplex128(a, 1<<logL)
		SliceBitReverseInPlaceComplex128(b, 1<<logL)
		SliceBitReverseInPlaceComplex128(c, 1<<logL)

		if len(a) > 1<<logL {
			SliceBitReverseInPlaceComplex128(a[1<<logL:], 1<<logL)
			SliceBitReverseInPlaceComplex128(b[1<<logL:], 1<<logL)
			SliceBitReverseInPlaceComplex128(c[1<<logL:], 1<<logL)
		}
	}

	addToDiagMatrix(vectors, 0, a)
	addToDiagMatrix(vectors, rot, b)
	addToDiagMatrix(vectors, (1<<logL)-rot, c)

	return
}

func genRepackMatrix(logL int, bitreversed bool) (vectors map[int][]complex128) {

	vectors = make(map[int][]complex128)

	a := make([]complex128, 2<<logL)
	b := make([]complex128, 2<<logL)

	for i := 0; i < 1<<logL; i++ {
		a[i] = complex(1, 0)
		a[i+(1<<logL)] = complex(0, 1)

		b[i] = complex(0, 1)
		b[i+(1<<logL)] = complex(1, 0)
	}

	addToDiagMatrix(vectors, 0, a)
	addToDiagMatrix(vectors, (1 << logL), b)

	return
}

func multiplyFFTMatrixWithNextFFTLevel(vec map[int][]complex128, logL, N, nextLevel int, a, b, c []complex128, forward, bitreversed bool) (newVec map[int][]complex128) {

	var rot int

	newVec = make(map[int][]complex128)

	if forward && !bitreversed || !forward && bitreversed {
		rot = (1 << (nextLevel - 1)) & (N - 1)
	} else {
		rot = (1 << (logL - nextLevel)) & (N - 1)
	}

	if bitreversed {
		SliceBitReverseInPlaceComplex128(a, 1<<logL)
		SliceBitReverseInPlaceComplex128(b, 1<<logL)
		SliceBitReverseInPlaceComplex128(c, 1<<logL)

		if len(a) > 1<<logL {
			SliceBitReverseInPlaceComplex128(a[1<<logL:], 1<<logL)
			SliceBitReverseInPlaceComplex128(b[1<<logL:], 1<<logL)
			SliceBitReverseInPlaceComplex128(c[1<<logL:], 1<<logL)
		}
	}

	/*
		fmt.Printf("W[%d]\n", nextLevel)
		for x := 0; x < 1<<logL; x++{
			if vec[x] != nil{
				fmt.Printf("%d :", x)
				for i := range vec[x] {
					fmt.Printf("%0.4f", vec[x][i])
				}
				fmt.Printf("\n")
			}

		}
		fmt.Printf("\n")

		fmt.Printf("W[%d]\n", nextLevel-1)
		fmt.Printf("%d :", 0)
		for i := range a {
			fmt.Printf("%0.4f", a[i])
		}
		fmt.Printf("\n")

		fmt.Printf("%d :", rot)
		for i := range b {
			fmt.Printf("%0.4f", b[i])
		}
		fmt.Printf("\n")

		fmt.Printf("%d :", N-rot)
		for i := range c {
			fmt.Printf("%0.4f", c[i])
		}
		fmt.Printf("\n")
		fmt.Println()
	*/

	for i := range vec {
		addToDiagMatrix(newVec, i, mul(vec[i], a))
		addToDiagMatrix(newVec, (i+rot)&(N-1), mul(rotate(vec[i], rot), b))
		addToDiagMatrix(newVec, (i-rot)&(N-1), mul(rotate(vec[i], -rot), c))
	}

	/*
		fmt.Printf("W[%d] x W[%d]\n", nextLevel, nextLevel-1)
		for x := 0; x < 1<<logL; x++{
			if newVec[x] != nil{
				fmt.Printf("%d :", x)
				for i := range newVec[x] {
					fmt.Printf("%0.4f", newVec[x][i])
				}
				fmt.Printf("\n")
			}

		}
		fmt.Printf("\n")
	*/

	return
}

func transposeDiagMatrix(mat map[int][]complex128, N int) {
	for i := range mat {
		if i < N>>1 {
			mat[i], mat[N-i] = mat[N-i], mat[i]
		}
	}
}

func conjugateDiagMatrix(mat map[int][]complex128) {
	for i := range mat {

		for j := range mat[i] {
			c := mat[i][j]
			mat[i][j] = complex(real(c), -imag(c))
		}
	}
}

func genBitReverseDiagMatrix(logN int) (diagMat map[int][]complex128) {

	var N, iRev, diff int

	diagMat = make(map[int][]complex128)

	N = 1 << logN

	for i := 0; i < N; i++ {
		iRev = int(utils.BitReverse64(uint64(i), uint64(logN)))

		diff = (i - iRev) & (N - 1)

		if diagMat[diff] == nil {
			diagMat[diff] = make([]complex128, N)
		}

		diagMat[diff][iRev] = complex(1, 0)
	}

	return
}

func addToDiagMatrix(diagMat map[int][]complex128, index int, vec []complex128) {
	if diagMat[index] == nil {
		diagMat[index] = vec
	} else {
		diagMat[index] = add(diagMat[index], vec)
	}
}

func rotate(x []complex128, n int) (y []complex128) {

	y = make([]complex128, len(x))

	mask := int(len(x) - 1)

	// Rotates to the left
	for i := 0; i < len(x); i++ {
		y[i] = x[(i+n)&mask]
	}

	return
}

func mul(a, b []complex128) (res []complex128) {

	res = make([]complex128, len(a))

	for i := 0; i < len(a); i++ {
		res[i] = a[i] * b[i]
	}

	return
}

func add(a, b []complex128) (res []complex128) {

	res = make([]complex128, len(a))

	for i := 0; i < len(a); i++ {
		res[i] = a[i] + b[i]
	}

	return
}
