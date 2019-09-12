#!/bin/bash

source ../harmony/scripts/setup_bls_build_flags.sh

# Decent commit is: b4c9a3264a3639367c9baab168aa8e5c7ab2715f
# from harmony repo (needed to check balances, etc)
s='one1tp7xdd9ewwnmyvws96au0e7e7mz6f8hjqr3g3p'
r='one1spshr72utf6rwxseaz339j09ed8p6f8ke370zj'

# --node http://s0.b.hmny.io:9500 \

function hb_test_issue() {
    ./hmy --node="https://api.s1.b.hmny.io" --pretty balance one1yc06ghr2p8xnl2380kpfayweguuhxdtupkhqzw
    HMY_RPC_DEBUG=true HMY_TX_DEBUG=true ./hmy --node="https://api.s0.b.hmny.io/" \
		 --pretty transfer --from one1yc06ghr2p8xnl2380kpfayweguuhxdtupkhqzw \
		 --account-name='acc2' --to one1q6gkzcap0uruuu8r6sldxuu47pd4ww9w9t7tg6 \
		 --from-shard 0 --to-shard 0 --amount 200
    # HMY_RPC_DEBUG=true HMY_TX_DEBUG=true  dlv exec -- ./hmy --node="https://api.s0.b.hmny.io/" \
    # 		 --pretty transfer --from one1yc06ghr2p8xnl2380kpfayweguuhxdtupkhqzw \
    # 		 --account-name='acc1' --to one1q6gkzcap0uruuu8r6sldxuu47pd4ww9w9t7tg6 \
    # 		 --from-shard 0 --to-shard 0 --amount 200

}

hb_test_issue


function check_balances() {
    HMY_RPC_DEBUG=true HMY_TX_DEBUG=true ./hmy_cli account ${s}
    HMY_RPC_DEBUG=true HMY_TX_DEBUG=true ./hmy_cli account ${r}
}

# printf '======Balances PRIOR to transfer======\n'
# check_balances

# HMY_RPC_DEBUG=true HMY_TX_DEBUG=true ./hmy_cli transfer \
# 	  --from-address=${s} \
# 	  --to-address=${r} \
# 	  --from-shard=0 \
# 	  --to-shard=2 \
# 	  --amount=10 \
# 	  --pretty

# sleep 5

# printf '======Balances AFTER transfer======\n'
# check_balances
