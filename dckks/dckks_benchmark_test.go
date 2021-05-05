package dckks

import (
	"encoding/json"
	"testing"

	"github.com/ldsec/lattigo/v2/ckks"
	"github.com/ldsec/lattigo/v2/drlwe"
	"github.com/ldsec/lattigo/v2/ring"
	"github.com/ldsec/lattigo/v2/rlwe"
)

func BenchmarkDCKKS(b *testing.B) {

	defaultParams := ckks.DefaultParams
	if testing.Short() {
		defaultParams = ckks.DefaultParams[:2]
	}
	if *flagParamString != "" {
		var jsonParams ckks.ParametersLiteral
		json.Unmarshal([]byte(*flagParamString), &jsonParams)
		defaultParams = []ckks.ParametersLiteral{jsonParams} // the custom test suite reads the parameters from the -params flag
	}

	for _, p := range defaultParams {

		params, err := ckks.NewParametersFromLiteral(p)
		if err != nil {
			panic(err)
		}

		var testCtx *testContext
		if testCtx, err = genTestParams(params); err != nil {
			panic(err)
		}

		benchPublicKeyGen(testCtx, b)
		benchRelinKeyGen(testCtx, b)
		benchKeySwitching(testCtx, b)
		benchPublicKeySwitching(testCtx, b)
		benchRotKeyGen(testCtx, b)
		benchRefresh(testCtx, b)
		benchRefreshAndPermute(testCtx, b)
	}
}

func benchPublicKeyGen(testCtx *testContext, b *testing.B) {

	sk0Shards := testCtx.sk0Shards

	crpGenerator := ring.NewUniformSampler(testCtx.prng, testCtx.dckksContext.ringQP)
	crp := crpGenerator.ReadNew()

	type Party struct {
		*CKGProtocol
		s  *rlwe.SecretKey
		s1 *drlwe.CKGShare
	}

	p := new(Party)
	p.CKGProtocol = NewCKGProtocol(testCtx.params)
	p.s = sk0Shards[0]
	p.s1 = p.AllocateShares()

	b.Run(testString("PublicKeyGen/Gen/", parties, testCtx.params), func(b *testing.B) {

		// Each party creates a new CKGProtocol instance
		for i := 0; i < b.N; i++ {
			p.GenShare(p.s, crp, p.s1)
		}
	})

	b.Run(testString("PublicKeyGen/Agg/", parties, testCtx.params), func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			p.AggregateShares(p.s1, p.s1, p.s1)
		}
	})

}

func benchRelinKeyGen(testCtx *testContext, b *testing.B) {

	sk0Shards := testCtx.sk0Shards

	type Party struct {
		*RKGProtocol
		ephSk  *rlwe.SecretKey
		sk     *rlwe.SecretKey
		share1 *drlwe.RKGShare
		share2 *drlwe.RKGShare
	}

	p := new(Party)
	p.RKGProtocol = NewRKGProtocol(testCtx.params)
	p.sk = sk0Shards[0]
	p.ephSk, p.share1, p.share2 = p.RKGProtocol.AllocateShares()

	crpGenerator := ring.NewUniformSampler(testCtx.prng, testCtx.dckksContext.ringQP)
	crp := make([]*ring.Poly, testCtx.params.Beta())

	for i := 0; i < testCtx.params.Beta(); i++ {
		crp[i] = crpGenerator.ReadNew()
	}

	b.Run(testString("RelinKeyGen/Round1Gen/", parties, testCtx.params), func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			p.GenShareRoundOne(p.sk, crp, p.ephSk, p.share1)
		}
	})

	b.Run(testString("RelinKeyGen/Round1Agg/", parties, testCtx.params), func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			p.AggregateShares(p.share1, p.share1, p.share1)
		}
	})

	b.Run(testString("RelinKeyGen/Round2Gen/", parties, testCtx.params), func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			p.GenShareRoundTwo(p.ephSk, p.sk, p.share1, crp, p.share2)
		}
	})

	b.Run(testString("RelinKeyGen/Round2Agg/", parties, testCtx.params), func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			p.AggregateShares(p.share2, p.share2, p.share2)
		}
	})

}

func benchKeySwitching(testCtx *testContext, b *testing.B) {

	sk0Shards := testCtx.sk0Shards
	sk1Shards := testCtx.sk1Shards

	type Party struct {
		*CKSProtocol
		s0    *rlwe.SecretKey
		s1    *rlwe.SecretKey
		share *drlwe.CKSShare
	}

	p := new(Party)
	p.CKSProtocol = NewCKSProtocol(testCtx.params, 6.36)
	p.s0 = sk0Shards[0]
	p.s1 = sk1Shards[0]
	p.share = p.AllocateShare()

	ciphertext := ckks.NewCiphertextRandom(testCtx.prng, testCtx.params, 1, testCtx.params.MaxLevel(), testCtx.params.Scale())

	b.Run(testString("KeySwitching/Gen/", parties, testCtx.params), func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			p.GenShare(p.s0, p.s1, ciphertext, p.share)
		}
	})

	b.Run(testString("KeySwitching/Agg/", parties, testCtx.params), func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			p.AggregateShares(p.share, p.share, p.share)
		}
	})

	b.Run(testString("KeySwitching/KS/", parties, testCtx.params), func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			p.KeySwitch(p.share, ciphertext, ciphertext)
		}
	})
}

func benchPublicKeySwitching(testCtx *testContext, b *testing.B) {

	sk0Shards := testCtx.sk0Shards
	pk1 := testCtx.pk1

	ciphertext := ckks.NewCiphertextRandom(testCtx.prng, testCtx.params, 1, testCtx.params.MaxLevel(), testCtx.params.Scale())

	type Party struct {
		*PCKSProtocol
		s     *rlwe.SecretKey
		share *drlwe.PCKSShare
	}

	p := new(Party)
	p.PCKSProtocol = NewPCKSProtocol(testCtx.params, 6.36)
	p.s = sk0Shards[0]
	p.share = p.AllocateShares(ciphertext.Level())

	b.Run(testString("PublicKeySwitching/Gen/", parties, testCtx.params), func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			p.GenShare(p.s, pk1, ciphertext, p.share)
		}
	})

	b.Run(testString("PublicKeySwitching/Agg/", parties, testCtx.params), func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			p.AggregateShares(p.share, p.share, p.share)
		}
	})

	b.Run(testString("PublicKeySwitching/KS/", parties, testCtx.params), func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			p.KeySwitch(p.share, ciphertext, ciphertext)
		}
	})
}

func benchRotKeyGen(testCtx *testContext, b *testing.B) {

	ringQP := testCtx.dckksContext.ringQP
	sk0Shards := testCtx.sk0Shards

	type Party struct {
		*RTGProtocol
		s     *rlwe.SecretKey
		share *drlwe.RTGShare
	}

	p := new(Party)
	p.RTGProtocol = NewRotKGProtocol(testCtx.params)
	p.s = sk0Shards[0]
	p.share = p.AllocateShares()

	crpGenerator := ring.NewUniformSampler(testCtx.prng, ringQP)
	crp := make([]*ring.Poly, testCtx.params.Beta())

	for i := 0; i < testCtx.params.Beta(); i++ {
		crp[i] = crpGenerator.ReadNew()
	}
	galEl := testCtx.params.GaloisElementForRowRotation()
	b.Run(testString("RotKeyGen/Round1/Gen/", parties, testCtx.params), func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			p.GenShare(p.s, galEl, crp, p.share)
		}
	})

	b.Run(testString("RotKeyGen/Round1/Agg/", parties, testCtx.params), func(b *testing.B) {

		for i := 0; i < b.N; i++ {
			p.Aggregate(p.share, p.share, p.share)
		}
	})

	rotKey := ckks.NewSwitchingKey(testCtx.params)
	b.Run(testString("RotKeyGen/Finalize/", parties, testCtx.params), func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			p.GenRotationKey(p.share, crp, rotKey)
		}
	})
}

func benchRefresh(testCtx *testContext, b *testing.B) {

	b.Run("Refresh/", func(b *testing.B) {

		if testCtx.params.MaxLevel() < 3 {
			b.Skip()
		}

		sk0Shards := testCtx.sk0Shards
		ringQ := testCtx.dckksContext.ringQ

		levelStart := 3

		type Party struct {
			*RefreshProtocol
			s      *ring.Poly
			share1 RefreshShareDecrypt
			share2 RefreshShareRecrypt
		}

		p := new(Party)
		p.RefreshProtocol = NewRefreshProtocol(testCtx.params)
		p.s = sk0Shards[0].Value
		p.share1, p.share2 = p.AllocateShares(levelStart)

		crpGenerator := ring.NewUniformSampler(testCtx.prng, ringQ)
		crp := crpGenerator.ReadNew()

		ciphertext := ckks.NewCiphertextRandom(testCtx.prng, testCtx.params, 1, levelStart, testCtx.params.Scale())

		b.Run(testString("Gen/", parties, testCtx.params), func(b *testing.B) {

			for i := 0; i < b.N; i++ {
				p.GenShares(p.s, levelStart, parties, ciphertext, testCtx.params.Scale(), crp, p.share1, p.share2)
			}
		})

		b.Run(testString("Agg/", parties, testCtx.params), func(b *testing.B) {

			for i := 0; i < b.N; i++ {
				p.Aggregate(p.share1, p.share1, p.share1)
			}
		})

		b.Run(testString("Decrypt/", parties, testCtx.params), func(b *testing.B) {

			for i := 0; i < b.N; i++ {
				p.Decrypt(ciphertext, p.share1)
			}
		})

		b.Run(testString("Recode/", parties, testCtx.params), func(b *testing.B) {

			for i := 0; i < b.N; i++ {
				p.Recode(ciphertext, testCtx.params.Scale())
			}
		})

		b.Run(testString("Recrypt/", parties, testCtx.params), func(b *testing.B) {

			for i := 0; i < b.N; i++ {
				p.Recrypt(ciphertext, crp, p.share2)
			}
		})

	})
}

func benchRefreshAndPermute(testCtx *testContext, b *testing.B) {

	b.Run("RefreshAndPermute/", func(b *testing.B) {

		if testCtx.params.MaxLevel() < 3 {
			b.Skip()
		}

		sk0Shards := testCtx.sk0Shards

		levelStart := 2

		type Party struct {
			*PermuteProtocol
			s      *ring.Poly
			share1 RefreshShareDecrypt
			share2 RefreshShareRecrypt
		}

		p := new(Party)
		p.PermuteProtocol = NewPermuteProtocol(testCtx.params)
		p.s = sk0Shards[0].Value
		p.share1, p.share2 = p.AllocateShares(levelStart)

		crpGenerator := ring.NewUniformSampler(testCtx.prng, testCtx.dckksContext.ringQP)
		crp := crpGenerator.ReadNew()

		ciphertext := ckks.NewCiphertextRandom(testCtx.prng, testCtx.params, 1, levelStart, testCtx.params.Scale())

		crpGenerator.Readlvl(levelStart, ciphertext.Value[0])
		crpGenerator.Readlvl(levelStart, ciphertext.Value[1])

		permutation := make([]uint64, testCtx.params.Slots())

		for i := range permutation {
			permutation[i] = ring.RandUniform(testCtx.prng, uint64(testCtx.params.Slots()), uint64(testCtx.params.Slots()-1))
		}

		b.Run(testString("Gen/", parties, testCtx.params), func(b *testing.B) {

			for i := 0; i < b.N; i++ {
				p.GenShares(p.s, levelStart, parties, ciphertext, crp, testCtx.params.Slots(), permutation, p.share1, p.share2)
			}
		})

		b.Run(testString("Agg/", parties, testCtx.params), func(b *testing.B) {

			for i := 0; i < b.N; i++ {
				p.Aggregate(p.share1, p.share1, p.share1)
			}
		})

		b.Run(testString("Decrypt/", parties, testCtx.params), func(b *testing.B) {

			for i := 0; i < b.N; i++ {
				p.Decrypt(ciphertext, p.share1)
			}
		})

		b.Run(testString("Permute/", parties, testCtx.params), func(b *testing.B) {

			for i := 0; i < b.N; i++ {
				p.Permute(ciphertext, permutation, testCtx.params.Slots())
			}
		})

		b.Run(testString("Recrypt/", parties, testCtx.params), func(b *testing.B) {

			for i := 0; i < b.N; i++ {
				p.Recrypt(ciphertext, crp, p.share2)
			}
		})
	})
}
