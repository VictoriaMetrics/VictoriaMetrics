package jwt

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"testing"
)

func TestParseJWKs_RSA(t *testing.T) {
	f := func(resp []byte) {
		t.Helper()

		vp, err := ParseJWKs(resp)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(vp.vs) != 1 {
			t.Fatalf("expected 1 key, got %d", len(vp.vs))
		}

		v0 := vp.vs[0]

		if v0.alg != "RS256" {
			t.Fatalf("expected RS256 algorithm, got %s", v0.alg)
		}
		if v0.kid == "" {
			t.Fatalf("expected non-empty kid")
		}

		k, ok := v0.key.(*rsa.PublicKey)
		if !ok {
			t.Fatalf("expected rsa.PublicKey, got %T", v0.key)
		}

		if k.E != 65537 {
			t.Fatalf("expected E=65537, got %d", k.E)
		}
	}

	f([]byte(`
{
    "keys": [
        {
            "kty": "RSA",
            "alg": "RS256",
            "use": "sig",
            "kid": "9c4d20a55ea37499b9f507f913566e5c",
            "e": "AQAB",
            "n": "yfAcjabVvhgdyNWI-2GDNX_yfrKI7dpBHJ1y2qTOTOFSZ1yN2d_bTiO-Xf6iB392xr7rfwmSUBDPqRY596PKAqVGeq7mgBBNBqJ8WJkBlbqXScAPgbzIH38TmaPilBczUIVOs426gdK3cBjFGarNssMqrn2DaNyW2VXNNUuC4yCcr5HLeChIHWejVFNJTVHrqE7ozYxt3YgNW8j3AuwIlUROxLCS8y-bgZtHLeiRcWGZpN6QeKAeWF28p_HdP1-N9_nznVlY09bpNQSWt_mdeuC7Jpy6ZkFX_396IOH9eG8OWku6Se091uEaJ8E96R1mg8MkOgCMOLt5H11AM9w5Bw8jlu_lWJTi-ugMU9hsEqkVeMLUN9UPiKzwtQbGotfaxkKqwcj5GAcTQMPdczgNykpA4RytwMS-2xkPvmb97Ezr6vbWrrfR_fY2Tu9WngTrJcMWCNri_8knIxeq6CpC8jrdzCdCcwB75PmZI6Jt-C18KYlfDEVmaOa0Kdd1yYobccIHBr2VL90BqrOyDiOscnQ4nBg5LA2haZ1QJXsvcjFka6bu1d2Q9PF3czEUjXMXQrId3hftOqJLRHjHgcc_mq5SgzLKmyXyHhRaaT90v7OnlJDEptrAyM6K3fqZvgKqbS0l94o6pwg6Pu5VpuKpy3oJjHUfjm8xbiaUcL13N6s"
        }
    ]
}
`))

	// Example key from https://www.rfc-editor.org/rfc/rfc7517#appendix-A.1
	f([]byte(`
{
    "keys": [
        {
            "kty": "RSA",
            "alg": "RS256",
            "kid": "2011-04-29",
            "e": "AQAB",
            "n": "0vx7agoebGcQSuuPiLJXZptN9nndrQmbXEps2aiAFbWhM78LhWx4cbbfAAtVT86zwu1RK7aPFFxuhDR1L6tSoc_BJECPebWKRXjBZCiFV4n3oknjhMstn64tZ_2W-5JsGY4Hc5n9yBXArwl93lqt7_RN5w6Cf0h4QyQ5v-65YGjQR0_FDW2QvzqY368QQMicAtaSqzs8KJZgnYb9c7d0zgdAZHzu6qMQvRL5hajrn1n91CbOpbISD08qNLyrdkt-bFTWhAI4vMQFh6WeZu0fM4lFd2NcRwr3XPksINHaQ-G_xBniIqbw0Ls1jF44-csFCur-kEgU8awapJzKnqDKgw"
        }
    ]
}
`))
}

func TestParseJWKs_EC(t *testing.T) {
	f := func(resp []byte) {
		t.Helper()

		vp, err := ParseJWKs(resp)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(vp.vs) != 1 {
			t.Fatalf("expected 1 key, got %d", len(vp.vs))
		}
		v0 := vp.vs[0]
		_, ok := v0.key.(*ecdsa.PublicKey)
		if !ok {
			t.Fatalf("expected ecdsa.PublicKey, got %T", v0.key)
		}
	}

	// Example key from https://www.rfc-editor.org/rfc/rfc7517#appendix-A.1
	f([]byte(`
{
    "keys": [
        {
            "kty": "EC",
            "kid": "1",
            "crv": "P-256",
            "y": "4Etl6SRW2YiLUrN5vfvVHuhp7x8PxltmWWlbbM4IFyM",
            "x": "MKBCTNIcKUSDii11ySs3526iDZ8AiTo7Tu6KPAqv7D4"
        }
    ]
}
`))
}

func TestParseMultipleKeys(t *testing.T) {
	// Microsoft JWKS keys
	// https://login.microsoftonline.com/common/discovery/v2.0/keys
	raw := []byte(`
{
  "keys": [
    {
      "kty": "RSA",
      "use": "sig",
      "kid": "sM1_yAxV8GV4yN-B6j2xzmik5Ao",
      "x5t": "sM1_yAxV8GV4yN-B6j2xzmik5Ao",
      "n": "pzQ8qb-1iwOmoaefhsaC22jjZ_u4AHsyTRDofsjXif9Zs8ACL8WlnlsFl5vRNUcN75-682CHeaOdhZxD4D20D0k9fRJA1WB-4FUZ9KVkjWyLPtY4VmjiVw7wsPwj018I1a1nmeEJME7BJvFOzqZGmX2GuZ8QkqZnQYCLnW5wobPRE009rlKql9c0VcFegzd-uAtATLslW568UaEAWyA4wdOKW_XC1YdIPFic8rtme-y_6tPK8Vb01pP7zauxXg84pGtWey-brc1JL4lmXKlRx7SNQCGn80kgbt1Vu05OOpA444-ckH0uU8kI6eZmghqOSt90EdvR0Lw47SALd-UfhQ",
      "e": "AQAB",
      "x5c": [
        "MIIC/jCCAeagAwIBAgIJAKnu4SM8gOE6MA0GCSqGSIb3DQEBCwUAMC0xKzApBgNVBAMTImFjY291bnRzLmFjY2Vzc2NvbnRyb2wud2luZG93cy5uZXQwHhcNMjYwMTI1MjEwMzQ4WhcNMzEwMTI1MjEwMzQ4WjAtMSswKQYDVQQDEyJhY2NvdW50cy5hY2Nlc3Njb250cm9sLndpbmRvd3MubmV0MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEApzQ8qb+1iwOmoaefhsaC22jjZ/u4AHsyTRDofsjXif9Zs8ACL8WlnlsFl5vRNUcN75+682CHeaOdhZxD4D20D0k9fRJA1WB+4FUZ9KVkjWyLPtY4VmjiVw7wsPwj018I1a1nmeEJME7BJvFOzqZGmX2GuZ8QkqZnQYCLnW5wobPRE009rlKql9c0VcFegzd+uAtATLslW568UaEAWyA4wdOKW/XC1YdIPFic8rtme+y/6tPK8Vb01pP7zauxXg84pGtWey+brc1JL4lmXKlRx7SNQCGn80kgbt1Vu05OOpA444+ckH0uU8kI6eZmghqOSt90EdvR0Lw47SALd+UfhQIDAQABoyEwHzAdBgNVHQ4EFgQU8KOkOebJetKfQWQMnMmIvxlkRB0wDQYJKoZIhvcNAQELBQADggEBABlckHkuXxRPlCY0wIFz2K6x5TBGfyXnffHUT2/0sMYC/5jC1bAxjUS2TfU2ziidolkJSsvEw34GvGyMZcySkNiOzVtSMi2kDYLQpHxBrR3nYftv8wUHazTq1VX2UDbPWIjl47CqEYjx/tboqttiWoe37zqp8tLkvRF6pd2UMVFGMoLa1Y2l1+kWrhQBfFngDvbj2Tuk556f17dyJB57babjL+KUZHXRY126z8Lt6Gs0bH3c5JJePJJbLdWcMx8mP3DKmzyva6Y5GYqG320SNfBB9rXxllJoFRc4zyRe52NKSZbphzUVWLank2Zxe3wvN+6zTXSpSJpU4zEA4bsvmlw="
      ],
      "cloud_instance_name": "microsoftonline.com",
      "issuer": "https://login.microsoftonline.com/{tenantid}/v2.0"
    },
    {
      "kty": "RSA",
      "use": "sig",
      "kid": "2wbgurt7VVnZZlR3EY6QlPdzlYo",
      "x5t": "2wbgurt7VVnZZlR3EY6QlPdzlYo",
      "n": "vMf5SDVxCoXm_9Q9XYhjB2G1f0C3INmriNirCSWZA3Cbm2hgCK8VfnRRbOgnHTvfoJDUkU_ujUkGmFwSFQzIlXrmzXhPcDcV0Xhuk___jqqX2Pghi1zumKwUuq-oQTfBChLOgv37qLHu3z1n0vPtXNdEYNKL4hPxrvbk84E1xvdnSnv5KUIt1ZEDi34xiEn2ug086FG7_I9QsPQ7ebazISan03nXC0yMwcIwI4GoRk0qDOuIU6Ba4sIudzAahtGVuI_cZlYODm6Df8O5mPnOZd1KoqzHfffn38ZnKpZEs2CB1xUQak3zUAe4eO2WW1xSHo3nvQiV9lyP6bO7t0ADTw",
      "e": "AQAB",
      "x5c": [
        "MIIC/jCCAeagAwIBAgIJANt8LDdsywkEMA0GCSqGSIb3DQEBCwUAMC0xKzApBgNVBAMTImFjY291bnRzLmFjY2Vzc2NvbnRyb2wud2luZG93cy5uZXQwHhcNMjYwMjA1MDcwMjUxWhcNMzEwMjA1MDcwMjUxWjAtMSswKQYDVQQDEyJhY2NvdW50cy5hY2Nlc3Njb250cm9sLndpbmRvd3MubmV0MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAvMf5SDVxCoXm/9Q9XYhjB2G1f0C3INmriNirCSWZA3Cbm2hgCK8VfnRRbOgnHTvfoJDUkU/ujUkGmFwSFQzIlXrmzXhPcDcV0Xhuk///jqqX2Pghi1zumKwUuq+oQTfBChLOgv37qLHu3z1n0vPtXNdEYNKL4hPxrvbk84E1xvdnSnv5KUIt1ZEDi34xiEn2ug086FG7/I9QsPQ7ebazISan03nXC0yMwcIwI4GoRk0qDOuIU6Ba4sIudzAahtGVuI/cZlYODm6Df8O5mPnOZd1KoqzHfffn38ZnKpZEs2CB1xUQak3zUAe4eO2WW1xSHo3nvQiV9lyP6bO7t0ADTwIDAQABoyEwHzAdBgNVHQ4EFgQUKItP7WFIpP7HiCxLoCKxEwI4mf8wDQYJKoZIhvcNAQELBQADggEBAJEAWl7Td/JZ99Dfk8Q6Q936uJWNbJd1Tzcoi8YWi2QrVUtTXGi80mpwi1iqjizlr6QJAicw7Xdw2noCJz1eYZeKe4X05ILUwNJh4plOMbWxa5IavXUOaDZLcv2PsGa1gmiUZYdQR7vmOVA8aSUlTqpj7h2pG2mmc/d5YVLXg1LhN+yGA3SnKolB489AhB76zJClzC5XVF+89rRpMmusxovsjNW+T7feJlHF3xi7W4hcIocSokxAXTkodNLdxtDndJA2EbPPhGJK19FE9fxGfUld09HbPZrFK0f9Xakmgj/TVkXJ/oJIsfofO4abCGnQ5KmpHU7XxctmQLXzBj4wzEI="
      ],
      "cloud_instance_name": "microsoftonline.com",
      "issuer": "https://login.microsoftonline.com/{tenantid}/v2.0"
    },
    {
      "kty": "RSA",
      "use": "sig",
      "kid": "QZgN9HqNkGNEM4GeKczD02PcVv4",
      "x5t": "QZgN9HqNkGNEM4GeKczD02PcVv4",
      "n": "idcSMX9CVqoiodieQh7OBbrY7crFiqsAtx-KjlpU7-0B9dyDW7zeiiPnx7SAb0J8fBmb7O12v4U4BHIJYSiFRxxGaFpOvparNwCJdxCJk69Ozs03MBcoO0p7cYQQZGCE8HjBF7z9mVUGIvgSYBvtwg7fUt2k-89itOgIzzpF8Jm9nBHNgSO8Zvv99pF1IfeHyVo0eITIUbQrPYvW5rA0eKyjRXygeKFFSSnaZweKOJmCdXX6undRzDObUP3rMbv-IFMDNsM4j_aBL-5vDVKpn7Au7MhBbr83xv6o6RFDWycljjTaGpYET_ueL_or-ehyf84ZRHVZpqOZefPNtZeZfw",
      "e": "AQAB",
      "x5c": [
        "MIIC/TCCAeWgAwIBAgIILaami+xa9gMwDQYJKoZIhvcNAQELBQAwLTErMCkGA1UEAxMiYWNjb3VudHMuYWNjZXNzY29udHJvbC53aW5kb3dzLm5ldDAeFw0yNjAyMjEwNTAzMDZaFw0zMTAyMjEwNTAzMDZaMC0xKzApBgNVBAMTImFjY291bnRzLmFjY2Vzc2NvbnRyb2wud2luZG93cy5uZXQwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCJ1xIxf0JWqiKh2J5CHs4FutjtysWKqwC3H4qOWlTv7QH13INbvN6KI+fHtIBvQnx8GZvs7Xa/hTgEcglhKIVHHEZoWk6+lqs3AIl3EImTr07OzTcwFyg7SntxhBBkYITweMEXvP2ZVQYi+BJgG+3CDt9S3aT7z2K06AjPOkXwmb2cEc2BI7xm+/32kXUh94fJWjR4hMhRtCs9i9bmsDR4rKNFfKB4oUVJKdpnB4o4mYJ1dfq6d1HMM5tQ/esxu/4gUwM2wziP9oEv7m8NUqmfsC7syEFuvzfG/qjpEUNbJyWONNoalgRP+54v+iv56HJ/zhlEdVmmo5l58821l5l/AgMBAAGjITAfMB0GA1UdDgQWBBQNpQkCxHV1tY+HilDxZ/NuMvS8BDANBgkqhkiG9w0BAQsFAAOCAQEATelFDXIrDxeJ+G+3ERppylvf/oEBkIsNnii8sg+zVltSJ4TC4OBrGC80vwDkxQVOGJBjYk9sYnMsHkKeYkFsCOK25DbhDB2GLhFnNUYzctPrwd/HLcFFgrNxM5xNvGdEQq5uRhELD0mJg3tfaWlVXOpLXifpjvE7sdT3wDzMl9S5iefqS00Qk3OwKPw9i9nfRsmawTaIQScxuX4RHS9K0Xjilr1K0FN8JSNXSLD7PjmBYcJqa/jeMsK+J7SyLXJr3rkMCD5UOsy18+QEpwzJhVtqgcOlKrgtN4W6JQCNfLWYUy1id6/YhIpJyU/9zHpfCR8TFNsMpdl0FG7IzrHYiA=="
      ],
      "cloud_instance_name": "microsoftonline.com",
      "issuer": "https://login.microsoftonline.com/{tenantid}/v2.0"
    },
    {
      "kty": "RSA",
      "use": "sig",
      "kid": "0KXs6yMcBGY0Kgs4pBP7wck44Vk",
      "x5t": "0KXs6yMcBGY0Kgs4pBP7wck44Vk",
      "n": "iLRVwHm_zFslYk3TAqM-ont1fxyvsUPfuc9W_zvAs-d0aGc1-iiUVWLUGsuPQ3kUuoW_GnXoeP2zGWqaKHWPAdLLttPtc2tvVcIWtbnc_wPpHM3BPwt64FjKlw4z5FTrVHmlClRZ61FvJCf1jTpI7VdoF06VtfDbamQH9j9OywrrUuh3A2-0CGKX8FYKilLoR1ZIJ01eKYIO79cxY9F2tjxocW0I61Hrqbj3EAdh1sD9ibKHKd5u1pvFarGhNXfzOK78pUVZCtaNgUxr2PzY5h7D67bePD8ULj88s3Jz6CvnDkh5n-nIA_ErtdHVtV5hz-00jaHtVM2NjPHN5O3ehQ",
      "e": "AQAB",
      "x5c": [
        "MIIC/TCCAeWgAwIBAgIIFySsKaILgHAwDQYJKoZIhvcNAQELBQAwLTErMCkGA1UEAxMiYWNjb3VudHMuYWNjZXNzY29udHJvbC53aW5kb3dzLm5ldDAeFw0yNjAzMDIxMjAzNDBaFw0zMTAzMDIxMjAzNDBaMC0xKzApBgNVBAMTImFjY291bnRzLmFjY2Vzc2NvbnRyb2wud2luZG93cy5uZXQwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCItFXAeb/MWyViTdMCoz6ie3V/HK+xQ9+5z1b/O8Cz53RoZzX6KJRVYtQay49DeRS6hb8adeh4/bMZapoodY8B0su20+1za29Vwha1udz/A+kczcE/C3rgWMqXDjPkVOtUeaUKVFnrUW8kJ/WNOkjtV2gXTpW18NtqZAf2P07LCutS6HcDb7QIYpfwVgqKUuhHVkgnTV4pgg7v1zFj0Xa2PGhxbQjrUeupuPcQB2HWwP2Jsocp3m7Wm8VqsaE1d/M4rvylRVkK1o2BTGvY/NjmHsPrtt48PxQuPzyzcnPoK+cOSHmf6cgD8Su10dW1XmHP7TSNoe1UzY2M8c3k7d6FAgMBAAGjITAfMB0GA1UdDgQWBBRyx0Mbnolq6hAcMxd9lE9M5tu2ujANBgkqhkiG9w0BAQsFAAOCAQEANnN3fo0oBEy86wBcPOx+bxwdb8SK3JFl4tZenwOCqHuJlC6e6MXyUZaZn/E5Do7otf3ehuNHCQePKBKnb/D54ccgrKEKKvOLUaN8YugJJiz/lbm6ZnA6DU+RBnUysmjCuVwnVO8xZidmnBuFh7ltVUzsWYgU7RGqns76rPWyxl1PglUOvTf0JvX7u8wlkmq4pA1QdNScVQ8e7oA2ri0G3GgA9CKuTPBx2tz+RTOBWQLkGU63/usEsfYXTbN8fUnt2UjP0ys8zONuor8txeWAx1+jEg5qf8PPZ7NpQ2Kw9KWJ1zDCuC/AlvZx1JMZE84g0qaPXlO1sz4Jg/VJbe8H6Q=="
      ],
      "cloud_instance_name": "microsoftonline.com",
      "issuer": "https://login.microsoftonline.com/{tenantid}/v2.0"
    },
    {
      "kty": "RSA",
      "use": "sig",
      "kid": "jKqmfMT6oL38waSSOcP1_5_KkAM",
      "x5t": "jKqmfMT6oL38waSSOcP1_5_KkAM",
      "n": "te3bGRsKDjomAcEVy1NPWbls_M_kzFEv1rzAE8DdJl-cl2je9IT4FCbLD2-hPhGIYXnNs_H-pMyPisHPGa3OXNHgF6pAIr1cRwykt5WdzFXywr5uKzVn-hdhWjBicsCdOYu0OlDs4CNc8HwRNKr_b3zHkKDSa5LG2J3xtmNqMFw6IE6daVVslX4MOx_U3WzOf4fHqTTSSEJ7aPCdNwcEhC94e5qGFV79AL6bHzg5lZ8AyE2u8rXSAYtuuAZ0RU_jtXRVO0MX3RjR299bJkPqB02ODIORGQ46e5K3WxApHY5oKxFzArYSuCvykODBkG2k7KQABax95eePTgMAnMGSuw",
      "e": "AQAB",
      "x5c": [
        "MIIC6TCCAdGgAwIBAgIIUPPNaxtnTWwwDQYJKoZIhvcNAQELBQAwIzEhMB8GA1UEAxMYbG9naW4ubWljcm9zb2Z0b25saW5lLnVzMB4XDTI2MDIwNDE3MDEzNloXDTMxMDIwNDE3MDEzNlowIzEhMB8GA1UEAxMYbG9naW4ubWljcm9zb2Z0b25saW5lLnVzMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAte3bGRsKDjomAcEVy1NPWbls/M/kzFEv1rzAE8DdJl+cl2je9IT4FCbLD2+hPhGIYXnNs/H+pMyPisHPGa3OXNHgF6pAIr1cRwykt5WdzFXywr5uKzVn+hdhWjBicsCdOYu0OlDs4CNc8HwRNKr/b3zHkKDSa5LG2J3xtmNqMFw6IE6daVVslX4MOx/U3WzOf4fHqTTSSEJ7aPCdNwcEhC94e5qGFV79AL6bHzg5lZ8AyE2u8rXSAYtuuAZ0RU/jtXRVO0MX3RjR299bJkPqB02ODIORGQ46e5K3WxApHY5oKxFzArYSuCvykODBkG2k7KQABax95eePTgMAnMGSuwIDAQABoyEwHzAdBgNVHQ4EFgQUcQ1LalAwjaodlOFImHjdY5SEq+QwDQYJKoZIhvcNAQELBQADggEBAEHTX+gnaZ1hoGmls5yFEQWOrIahSBjC1WuBKmRTDW6FV7jjRyIxdhQgsRFYvMpAioNnDlvOIyA90SflIF035XdWrHv/4ROtq+xvsNgtV3D4AB5mr4VPG0OvpTwfLt4WXzD33nFsf42wRAe1MASB2Y2mw6rarUC9d+oPqd/qXfkYoX8oUPW6RfkD/V3oEW9dU+Et8dAobxqBbIN+Fncvlm4jTFR6WsMsdjaBOh085PipB96Fyh1PCFRRNZ9zzdomCMid6sVkTwmOyn12g7YpmC1DQzQ3/91OX826L98fun2AIjOZP2X9Re8kMb/TmTTzeTHNeXyDL9dUT51dEYhCWbA="
      ],
      "cloud_instance_name": "microsoftonline.us",
      "issuer": "https://login.microsoftonline.com/{tenantid}/v2.0"
    },
    {
      "kty": "RSA",
      "use": "sig",
      "kid": "IAzQT0lDZrQ9kAuCfAsMp74YGkA",
      "x5t": "IAzQT0lDZrQ9kAuCfAsMp74YGkA",
      "n": "r0MUNBLlVJlhOzSMz1gq7fKQ-VtRBGo5e0JoljylUqL14kqDaHwayhIc2o_Vx6sdo22p8-JQMPnBqMllGdgHZfNK28ROMvbdpRmWx4SdDgtmm9pTuZSCsZAh6IMT-MV7h5MqfqeoURhoqjScz5jLsGziO7fRvBrL0JPWrgk21XGguUGbdKRxBzlMTY4-5JPQ0GmJz81BQ4JVs6TB1flNSIQtjb2zC2q-fgWZBmhgmjSTgHDppiwIYUFKagQcxdFuVQM2HNlbWfELjiatlV1wgv-4SjEVDbuXRV7-EElfnKv4RgFlZFFqDcDbJqXlhnkBt_bZ1WH9c3QQcBoE2G0CBw",
      "e": "AQAB",
      "x5c": [
        "MIIC6jCCAdKgAwIBAgIJALXyS77AxpNPMA0GCSqGSIb3DQEBCwUAMCMxITAfBgNVBAMTGGxvZ2luLm1pY3Jvc29mdG9ubGluZS51czAeFw0yNjAyMjIxNzAwMzZaFw0zMTAyMjIxNzAwMzZaMCMxITAfBgNVBAMTGGxvZ2luLm1pY3Jvc29mdG9ubGluZS51czCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAK9DFDQS5VSZYTs0jM9YKu3ykPlbUQRqOXtCaJY8pVKi9eJKg2h8GsoSHNqP1cerHaNtqfPiUDD5wajJZRnYB2XzStvETjL23aUZlseEnQ4LZpvaU7mUgrGQIeiDE/jFe4eTKn6nqFEYaKo0nM+Yy7Bs4ju30bway9CT1q4JNtVxoLlBm3SkcQc5TE2OPuST0NBpic/NQUOCVbOkwdX5TUiELY29swtqvn4FmQZoYJo0k4Bw6aYsCGFBSmoEHMXRblUDNhzZW1nxC44mrZVdcIL/uEoxFQ27l0Ve/hBJX5yr+EYBZWRRag3A2yal5YZ5Abf22dVh/XN0EHAaBNhtAgcCAwEAAaMhMB8wHQYDVR0OBBYEFG+C3O9dX6jgCqwTGFkyIoWutm1WMA0GCSqGSIb3DQEBCwUAA4IBAQAwQAu3uzx782oGJov1kxv4kISw33up1g0MAWyr3Jn9eI0/b94TD4LCuGU+tcac/4YItYLb9fS9J3Vm3wPRYUFY1f31hDFBq4DfUR6MjZreWz/8w4n+vK+Vbjx0JJS2y/evqLjCQvPfuRrdnNltXPLhpOk1OHSD1fzgJsWHwTvm/SZtWFGwQVU1Tf/gbF6li/bAZ0HLawHhJFB+8tzGK+Rch4dhvrRjxMC3o3LpD8uZb0SDlv7j2Wu8liiT7juumoE+DKuVzW4z+6DJEokaysFVvnCVVMzYiHaOKwag4lVwqCaFSTWXMP3eDkRwI6AgwrMoDcSUZqom77QveVFf2J5L"
      ],
      "cloud_instance_name": "microsoftonline.us",
      "issuer": "https://login.microsoftonline.com/{tenantid}/v2.0"
    },
    {
      "kty": "RSA",
      "use": "sig",
      "kid": "k4zFrp29E_BwFxDFO2POABSYaPE",
      "x5t": "k4zFrp29E_BwFxDFO2POABSYaPE",
      "n": "xqExU-10Y0VbAbyvZm4dJc8i3UQv8EJFl1xUWLlBwz4TuYnoPi4-mBFWSGgxITwHOhIYgXqElRG9Q_dwAyLwHNLcNFlY8NAp8uRLprr_OIbZ11uMk5pQPcuVwYK42bE2QDzsc_kSJcBIJYqCJ7Yo4EHYtLonl1sA9YuTeHXX121AgNIu7xrny68v7byjitk2JjpqOOE99d9AbOaoa-8EfUwq0cVwXMaN2fJyxM3RqajXrQHdf963q0iC1fkTWeWu6z4D1EyIra4bxKeCnFOlAzppPQEZpMPo7fZ-7c-uY5ZtAOBUJRKgeTaqps-ay5YdhWi1PXbcgtXpO3KGEYP-2Q",
      "e": "AQAB",
      "x5c": [
        "MIIDCzCCAfOgAwIBAgIRAMrLjv9zRXDW1wfXY4dTbFQwDQYJKoZIhvcNAQELBQAwKTEnMCUGA1UEAxMeTGl2ZSBJRCBTVFMgU2lnbmluZyBQdWJsaWMgS2V5MB4XDTI2MDIyMzExMzEzM1oXDTMxMDIyMzExMzEzM1owKTEnMCUGA1UEAxMeTGl2ZSBJRCBTVFMgU2lnbmluZyBQdWJsaWMgS2V5MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxqExU+10Y0VbAbyvZm4dJc8i3UQv8EJFl1xUWLlBwz4TuYnoPi4+mBFWSGgxITwHOhIYgXqElRG9Q/dwAyLwHNLcNFlY8NAp8uRLprr/OIbZ11uMk5pQPcuVwYK42bE2QDzsc/kSJcBIJYqCJ7Yo4EHYtLonl1sA9YuTeHXX121AgNIu7xrny68v7byjitk2JjpqOOE99d9AbOaoa+8EfUwq0cVwXMaN2fJyxM3RqajXrQHdf963q0iC1fkTWeWu6z4D1EyIra4bxKeCnFOlAzppPQEZpMPo7fZ+7c+uY5ZtAOBUJRKgeTaqps+ay5YdhWi1PXbcgtXpO3KGEYP+2QIDAQABoy4wLDAdBgNVHQ4EFgQUvmSlvAma7n9cmgjqR3MPcFtXEwYwCwYDVR0PBAQDAgLEMA0GCSqGSIb3DQEBCwUAA4IBAQB1z6SKorx1xQIdwz+yv42VzKGdAPrPOoVq5Pe8Y2R+AEwCoF4aRxdxnyae6264a0kUSs+tubfY1uhFYIEH3vCDGtqCyuaFwqr+5blExipycZa1eMrWlOa3jDulXYg+FSP74wNyddNxbI3778LTTFT953c1CltvYgCNlXKgKZkglsc4L2EnAet4XhfZbY0Q5eP5CkgyM5sHLP6ywjXfIOFEwzVxcNlj2dcINIfjkZh5A7kpEBTwT7gPoUGWrBe+JbNpzkvFpKOmuQH29A38BtRaJGQ59MfFblqrqTGyMEvWNsAV/aQ7Qcu/cQZQp1tDc+YaKkEqPDz5m3O67yDT8Dkw"
      ],
      "cloud_instance_name": "microsoftonline.com",
      "issuer": "https://login.microsoftonline.com/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0"
    },
    {
      "kty": "RSA",
      "use": "sig",
      "kid": "RKyMV69ovghTbBgMZM9O03u-jlM",
      "x5t": "RKyMV69ovghTbBgMZM9O03u-jlM",
      "n": "ktNw1FUwp2-kHs5nNi5ip11EbfXuhpfUyPoLbtJdEvZ5cVaGv3UMb4lW4cEmwPeCuuW4NFZAg_x4BP8px9bFMRUyuyE-tqm5-wst_VSjeaoVwBhZl3GinB7TOMj4t5U6p_7JcvpEN50hApsll511hHnOt9Eenskh_tcDReD1A9QTpAw09A-Cqh-LgSMHk3_fbJoW_Iy-xtsZUQ3iC5NrILX8ZbWgfJ8WLI-9-zXBLA_mc_HrV76gCFDE_KCQ7eYwhBZrSYZxuyE5sZ107Zjx6Tj_a1mACZ4M17TfwQhadvd9ex12sA1XvZfzgOrva8ZQ9bs1SAFXSXpxPttW3xQRmQ",
      "e": "AQAB",
      "x5c": [
        "MIIDCjCCAfKgAwIBAgIQXPfDq4JyVkx06NPyNi2KMDANBgkqhkiG9w0BAQsFADApMScwJQYDVQQDEx5MaXZlIElEIFNUUyBTaWduaW5nIFB1YmxpYyBLZXkwHhcNMjYwMjEwMTYzMDUxWhcNMzEwMjEwMTYzMDUxWjApMScwJQYDVQQDEx5MaXZlIElEIFNUUyBTaWduaW5nIFB1YmxpYyBLZXkwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQCS03DUVTCnb6Qezmc2LmKnXURt9e6Gl9TI+gtu0l0S9nlxVoa/dQxviVbhwSbA94K65bg0VkCD/HgE/ynH1sUxFTK7IT62qbn7Cy39VKN5qhXAGFmXcaKcHtM4yPi3lTqn/sly+kQ3nSECmyWXnXWEec630R6eySH+1wNF4PUD1BOkDDT0D4KqH4uBIweTf99smhb8jL7G2xlRDeILk2sgtfxltaB8nxYsj737NcEsD+Zz8etXvqAIUMT8oJDt5jCEFmtJhnG7ITmxnXTtmPHpOP9rWYAJngzXtN/BCFp29317HXawDVe9l/OA6u9rxlD1uzVIAVdJenE+21bfFBGZAgMBAAGjLjAsMB0GA1UdDgQWBBRpG3DOLNpVhLHLECM+WAhMISVoADALBgNVHQ8EBAMCAsQwDQYJKoZIhvcNAQELBQADggEBAGnFwRcKrJmmFSs+VxxUF+/hzlKs9inv/9wdvgp/5gklCNgftW1FSh5Z/wCXoFoeiIgwAHwTwH9y5zwDyYuewgfbYtOrHXhqpxhp4Wc9QlFe2tAjD9RQXsEuO3LxwEY0/FhFD4dpHeKyAN/cuuSy5Wvv51SsRcsPYPBjlSFhk9bYPCxmMs8j7QgWFf6GJXghgrGzmvEvqKilW3v9jKfet6hhTfUcbFXmDE2X7vZkJB6FvaRy8Velzw1UGwkF5lcwgDOE3qHjuSvREZ7wKCHpeXuIXWRPR7QYYEsK1NyAdGe7EwsG8bfjQ6MwKIUMjyviBS1NDBmgC4w1Qpgyl+Fg+/Y="
      ],
      "cloud_instance_name": "microsoftonline.com",
      "issuer": "https://login.microsoftonline.com/9188040d-6c67-4c5b-b112-36a304b66dad/v2.0"
    }
  ]
}`)
	vp, err := ParseJWKs(raw)
	if err != nil {
		t.Fatalf("error parsing jwks keys: %v", err)
	}

	// algo not set so vp.vs = len(jwks) * (len(rsaAlgs) + len(psAlgs))
	if len(vp.vs) != 48 {
		t.Fatalf("expected 48 keys, got %d", len(vp.vs))
	}

	for _, v := range vp.vs {
		if _, ok := v.key.(*rsa.PublicKey); !ok {
			t.Fatalf("expected rsa key, got %T", v.key)
		}
	}
}
