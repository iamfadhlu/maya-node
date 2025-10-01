package tss

import (
	"encoding/base64"

	"github.com/tendermint/tendermint/crypto"
	ctypes "gitlab.com/thorchain/binance-sdk/common/types"
	"gitlab.com/thorchain/binance-sdk/keys"
	"gitlab.com/thorchain/binance-sdk/types/tx"

	"gitlab.com/mayachain/mayanode/common"
)

// MockThorchainKeymanager is to mock the TSS , so as we could test it

type MockMayachainKeyManager struct{}

// keys.KeyManager interface methods

func (k *MockMayachainKeyManager) Sign(tx.StdSignMsg) ([]byte, error) {
	return nil, nil
}

func (k *MockMayachainKeyManager) GetPrivKey() crypto.PrivKey {
	return nil
}

func (k *MockMayachainKeyManager) GetAddr() ctypes.AccAddress {
	return nil
}

func (k *MockMayachainKeyManager) ExportAsMnemonic() (string, error) {
	return "", nil
}

func (k *MockMayachainKeyManager) ExportAsPrivateKey() (string, error) {
	return "", nil
}

func (k *MockMayachainKeyManager) ExportAsKeyStore(password string) (*keys.EncryptedKeyJSON, error) {
	return nil, nil
}

func (k *MockMayachainKeyManager) SignWithPool(msg tx.StdSignMsg, poolPubKey common.PubKey) ([]byte, error) {
	return nil, nil
}

// ThorchainKeyManager interface added methods (SignWithPool, RemoteSign)

func (k *MockMayachainKeyManager) RemoteSign(msg []byte, algo common.SigningAlgo, poolPubKey string) ([]byte, []byte, error) {
	// this is the key we are using to test TSS keysign result in BTC chain
	// tmayapub1addwnpepqwznsrgk2t5vn2cszr6ku6zned6tqxknugzw3vhdcjza284d7djp59sf99q
	if poolPubKey == "tmayapub1addwnpepqwznsrgk2t5vn2cszr6ku6zned6tqxknugzw3vhdcjza284d7djp59sf99q" {
		msgToSign := base64.StdEncoding.EncodeToString(msg)
		if msgToSign == "wqYuqkdeLjxtkKjmeAK0fOZygdw8zZgsDaJX7mrqWRE=" {
			sig, err := getSignature("ku/n0D18euwqkgM0kZn0OVX9+D7wfDBIWBMya1SGxWg=", "fw0sE6osjVN6vQtr9WxFrOpdxizPz9etSTOKGdjDY9A=")
			return sig, nil, err
		} else {
			sig, err := getSignature("256CpfiML7BDP1nXqKRc3Fq01PALeKwpXYv9P/H3Xhk=", "LoX6cVND0JN8bbZSTsoJcwLCysAKhyYtB2BFM3sdP98=")
			return sig, nil, err
		}
	}
	if poolPubKey == "tmayapub1addwnpepqw2k68efthm08f0f5akhjs6fk5j2pze4wkwt4fmnymf9yd463puru38eqd3" {
		msgToSign := base64.StdEncoding.EncodeToString(msg)
		switch msgToSign {
		case "BMxXf+K+1dYu3qGgvH59GXoxwwFfTnLjB7hHf3qflPk=":
			sig, err := getSignature("WGSFUPPCN0kTcXcylAIQXyAxO7OUC5YRjDRz9wmzpkk=", "RUIoqdza5Od9nMfU2teqbZJAeC+pTyHIbKq+72jJMfM=")
			return sig, nil, err
		case "7zpXFp0KDBebXPNc2ZGim8NQAY7GMwS7iwr4hl2tFZQ=":
			sig, err := getSignature("tCR9TWnSxn/HPr0T3I9XeneJ0dRmi2DqbOkcFPWIkNs=", "VAxipOj6ogfBci+WwJy4n9QfAjjhJk6WhQ1I8n6xEo4=")
			return sig, nil, err
		case "isIqvmEs/otDI3NC2C8zFr1DGu3k/p8g/1RdlE0KzBI=":
			sig, err := getSignature("Nkb9ZFkPpSi1i/GaJe6FkMZmx1IH2oDtnr0jGsycBF8=", "ZAQ0qbPtPtdAin5HVOMmMO6oJxwWT4T0GvqpeyGG168=")
			return sig, nil, err
		case "CpmfPxDQ7ELrAU4NsJ/9Bn6iqxHFqmqma7jxPUI0/Hk=":
			sig, err := getSignature("LgNsk6Fa588SunfG/PJlq/A9sZVzS7W0KBepvpEHuXE=", "UPV4LmfKyq0KdRoU563nwSkJIWTqCtt8VyKEVVRxX+I=")
			return sig, nil, err
		case "lrXGZ98PjMwkCVYLYHkBuWYJxCxc8lRHR0pkz/xNgeg=":
			sig, err := getSignature("FZ/zJ8UI2z7nhBCp8/YTdvkVgk6xVj0FfZV79ZEr+q8=", "J9u9gp+1tnZsS8evDsLhvq21v89bB92FvP5PDD+2WTk=")
			return sig, nil, err
		default:
			sig, err := getSignature("gVxKdVgWR+4OZLxFAu5uWWOPCxhGPqFtQAyVujqSuh8=", "JXPU4Li4spnonssxJS52r/hEBwt1iPFlvjwu8ZOe+F0=")
			return sig, nil, err
		}
	}
	if poolPubKey == "tmayapub1addwnpepqf2w20d7yxdw0fdr6zx46kpq3qfwud4ejq945e9eru0d9zc59785xm433h0" {
		msgToSign := base64.StdEncoding.EncodeToString(msg)
		if msgToSign == "PIZUt687khEYQizRpYbLyQgDw1Ou+xzbSrLQ8fTKiaw=" {
			sig, err := base64.StdEncoding.DecodeString("HxT9xOyBYuhHfK8iLSbPniJq6u6KYfJVmq28iO+/Sa44ocAuckpzs3g6zBelr4pUaxatoKixAaPt2UtlgPP2sA==")
			return sig, nil, err
		}
	}
	// ZEC test pubkeys & msgs
	if poolPubKey == "tmayapub1addwnpepqf2k093fjdpp428msc3zzecvmqxqr80lkwpx4arax3ac3q4kupsp5sl0wjg" {
		// pkwif: "cNPJRpojP7Fd8Gp3poQ5ChWD8SVMmozHmKzUukusPEJhZwyAWPaj", tag: "alone"
		msgToSign := base64.StdEncoding.EncodeToString(msg)
		switch msgToSign {
		case "QGkfmBY+cFJxRmowCz5ws1AiqJiEqA1m8GfMaEHJjQc=": // toAddress = "tmNUFAr71YAW3eXetm8fhx7k8zpUJYQiKZP"
			sig, err := getSignature("WJVJHn8jx2I/oSCJwVuDjnUK+u/pZRaDY6OD9BqTtfc=", "FA3MS8ulA+aCXWDZ5y2XdiPLLlAP6QBD9/ynGvFW4XQ=")
			return sig, nil, err
		case "/BmFjPQbaAWg4OQmLgD2ZGhjjF3Kk8zmCCzy9N2HqBU=": // toAddress = "tmWJQXMC6h9hh3ohjxnnS25o3pEFe1HR8zr"
			sig, err := getSignature("rlolQD6vtgQo3FOrPbzVKTUXIO24tydvVCIs+1QdNy4=", "a4hGMPAgi/hTFTCTEUhQhdR8Jct2z8kGOu/Ns3fpaJo=")
			return sig, nil, err
		}
	}
	if poolPubKey == "tmayapub1addwnpepqgk5v3txwyfuh6kah6zrplw9sj7svpvfd82gpgyv7zaqu0qkcr9xqmpvc7c" {
		// pkwif: "cQaw235gKVrNrw9x1uDBwGGsqJgMCMvWLnmaYdcnf7dS1wEsLjAX", tag: "tent"
		msgToSign := base64.StdEncoding.EncodeToString(msg)
		switch msgToSign {
		case "BErXh/iSXy47Aw2w7wNhfD8a7pxWXfjncrndXqHQu40=": // toAddress = "tmNUFAr71YAW3eXetm8fhx7k8zpUJYQiKZP"
			sig, err := getSignature("qb2Q5QRtOxVg7t9hRZ/WuqDWPfafgdAaamqP4ZH6+sQ=", "dW/XaEn62HBPyy8AasooV0WccR4mOijBbwvpYsbH7Ws=")
			return sig, nil, err
		case "BKRHpqqFpV31fBn3ErlKLApJpYDrKTn0UOBFYqokT24=": // toAddress = "tmWJQXMC6h9hh3ohjxnnS25o3pEFe1HR8zr"
			sig, err := getSignature("DboWXD3ai3tk/lDllDYK2vFFJMMfM2az5U9LISakx5Y=", "GPbsXwrQ23MoNlNZPAfoSI0czUqsFyzm5NU7/n5xfC4=")
			return sig, nil, err
		}
	}

	// ZEC test pubkeys & msgs - height 185
	switch poolPubKey {
	case "tmayapub1addwnpepqw35p34cta9gt37k837xqs3h499n3zl9gaegkq4vn0v5fjyn47j4kuutge7":
		// pkwif: "cQ24hdAZXPXSP5DHmRADhLEYQ67NsTSpZS57AsDNM2wiho4n1CvQ", tag: "wallet02"
		msgToSign := base64.StdEncoding.EncodeToString(msg)
		switch msgToSign {
		case "5R7ns7I6fHM+GS8rSU0IHEjNfYt0aEBxYdEeW13TVVM=": // toAddress = "tmNUFAr71YAW3eXetm8fhx7k8zpUJYQiKZP"
			sig, err := getSignature("35LUtRTbWKPnvn/cS34wVL6pqvf10rRB9oz1ZQfkyuQ=", "R/HFwKmf6eU+TnwcIBNzfjae28ZZcryH9EPnO9UK6Uk=")
			return sig, nil, err
		case "O69Kkk5eNbCjwdLvxzzkRvrcUPPKW0/Mm7rcl4Qrnok=": // toAddress = "tmWJQXMC6h9hh3ohjxnnS25o3pEFe1HR8zr"
			sig, err := getSignature("Iul00gqO+JkZFe5w/1/quvRlpBi+B+bXlMSj9sRScD4=", "T409xr1Maa7byCAMLvqbdoGNmyff70fImJD9za52qwI=")
			return sig, nil, err
		}

	case "tmayapub1addwnpepqgwg2uc7njp33myd9yj8en7sa3tft9fmfhuh36zs0eqthm00wt7tkahw5jy":
		// pkwif: "cT5r3oWRA7aezz7EA9h68QzscJkL2P69faJ5hrZN35RRp5C5eZYK", tag: "wallet03"
		msgToSign := base64.StdEncoding.EncodeToString(msg)
		switch msgToSign {
		case "k6gduCSvqTxzexbnP0kqhyvSTkbDlCve2T50SDdcXfo=": // toAddress = "tmNUFAr71YAW3eXetm8fhx7k8zpUJYQiKZP"
			sig, err := getSignature("F/48TOjqNYaVqPWXK1+a5R8PmI4pR9SY3f1Nd5u/7GA=", "S/X9dPlHYyE4hK3o0/enmbb60yHlv+0GilnseOha4Rc=")
			return sig, nil, err
		case "NOt8pmy85xIYQbRLDGI/3JCsSAvieM9aFj0RbFkqZME=": // toAddress = "tmWJQXMC6h9hh3ohjxnnS25o3pEFe1HR8zr"
			sig, err := getSignature("kwe1D+NaDhOQoqZeT2aIk5/1JvzeD/1Qt2XgT5tnSQk=", "WjN3VXR3WE6nAhRLEQD5wWhTZwY9szGcqZe1lya/tBc=")
			return sig, nil, err
		}
	}

	// ZEC test pubkeys & msgs - height 186
	switch poolPubKey {
	case "tmayapub1addwnpepqw35p34cta9gt37k837xqs3h499n3zl9gaegkq4vn0v5fjyn47j4kuutge7":
		// pkwif: "cQ24hdAZXPXSP5DHmRADhLEYQ67NsTSpZS57AsDNM2wiho4n1CvQ", tag: "wallet02"
		msgToSign := base64.StdEncoding.EncodeToString(msg)
		switch msgToSign {
		case "L2r13lQUvY07dKiLIVYeGBpyy2PX03lKbvZfh0GrNXg=": // toAddress = "tmNUFAr71YAW3eXetm8fhx7k8zpUJYQiKZP"
			sig, err := getSignature("BNLWTvNHyo0JZ8hZIv0I9gmwUI9gAWSPafJxO9UyC3M=", "OoVoe2ynAd4ne5v+sRpqt2jCHnQt8iGeFSOLAGvTa0Y=")
			return sig, nil, err
		case "fC3YTBYAKYFBWrZwKD7C5tARYUjJdQYM4jApOYwEJoY=": // toAddress = "tmWJQXMC6h9hh3ohjxnnS25o3pEFe1HR8zr"
			sig, err := getSignature("9aHpgDnVBuNhyGcH1yFbrGK4GohgGlwj+DQdF6oHbzE=", "cOo8ld4W9Hwx+TjuVf0sZlK5UGvHcsw/KKjylKUG4j4=")
			return sig, nil, err
		}

	case "tmayapub1addwnpepqgwg2uc7njp33myd9yj8en7sa3tft9fmfhuh36zs0eqthm00wt7tkahw5jy":
		// pkwif: "cT5r3oWRA7aezz7EA9h68QzscJkL2P69faJ5hrZN35RRp5C5eZYK", tag: "wallet03"
		msgToSign := base64.StdEncoding.EncodeToString(msg)
		switch msgToSign {
		case "uM5jnQx8xjkApVP+M6EB1XFF1nu7e4hoyq5iBeM5Z6w=": // toAddress = "tmNUFAr71YAW3eXetm8fhx7k8zpUJYQiKZP"
			sig, err := getSignature("xNEqAf2TQUBOkRz+xNwM/TbzytLEqM0QTa48qi1+z50=", "EDpJgLK8fOAXiwA6jtTQXzCT5mPaH6T1hZPoGJZBFMw=")
			return sig, nil, err
		case "IIkm3yxFLdu6pum0fVMXK+mPkC4MMxwMx79MsmVYUfM=": // toAddress = "tmWJQXMC6h9hh3ohjxnnS25o3pEFe1HR8zr"
			sig, err := getSignature("oM8SqxjEUsV+tvorGme0m+vRguJ7tRe6sk+kmfkudHI=", "WiDNgH4yWi+V393zu5EXEyS5XKaDP5nMRlnZmM5NYqA=")
			return sig, nil, err
		}
	}

	return nil, nil, nil
}
