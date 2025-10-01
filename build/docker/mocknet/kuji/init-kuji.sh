#!/bin/sh
if [ -f /root/.kujira/config/genesis.json ]; then
  exec /entrypoint.sh
  exit 0
fi

# init chain
/kujirad init local --chain-id harpoon-2 --default-denom ukuji

# create keys
cat <<EOF | /kujirad keys add master --recover --keyring-backend test
dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog dog fossil
EOF

# create genesis accounts
/kujirad genesis add-genesis-account kujira1hnyy4gp5tgarpg3xu6w5cw4zsyphx2ly3j7zzw 1000000000ukuji                                                                                                    # validator
/kujirad genesis add-genesis-account kujira1y4lj8cg47kfm70nht5f8ajyvr4dftfc6lmvga7 150000000000000factory/kujira1qk00h5atutpsv900x202pxx42npjr9thg58dnqpa72f2p7m2luase444a7/uusk,150000000000000ukuji # smoke contrib
/kujirad genesis add-genesis-account kujira1qr79m0r8hzj0t88c3c6k99prxv0fe34ulfzyzk 150000000000000factory/kujira1qk00h5atutpsv900x202pxx42npjr9thg58dnqpa72f2p7m2luase444a7/uusk,150000000000000ukuji # smoke master

/kujirad genesis gentx master 100000000ukuji --keyring-backend test --chain-id harpoon-2
/kujirad genesis collect-gentxs

# remove app.toml to enable api and listen on all interfaces (entrypoint.sh will copy it)
rm -f /root/.kujira/config/app.toml

exec /entrypoint.sh
