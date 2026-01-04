import { ethers, network } from "hardhat";
import { readConfig } from "../helper";
import type { BPoSPool } from "../../typechain-types";

type OperationType = 0 | 1 | 2; // Add = 0, Update = 1, Remove = 2

interface NodeOperationInput {
    operationType: OperationType;
    nickName?: string;
    ownerPublicKey: string;
    dposPublicKey?: string;
    votes?: string | number | bigint;
}

/**
 * Usage:
 * npx hardhat run scripts/pool/syncNodes.ts --network <net>
 *    --ops '[{"operationType":0,"nickName":"nodeA","ownerPublicKey":"0x..","dposPublicKey":"0x..","votes":"100"}]'
 */
async function main() {
    const [caller] = await ethers.getSigners();

    const bposPoolAddress = await readConfig(network.name, "BPoSPool");
    if (!bposPoolAddress) {
        throw new Error(`BPoSPool address not found for network ${network.name}`);
    }

    console.log("Network:", network.name);
    console.log("Caller:", caller.address);
    console.log("BPoSPool:", bposPoolAddress);

    const pool = (await ethers.getContractAt(
        "BPoSPool",
        bposPoolAddress,
        caller
    )) as BPoSPool;

    // Fallback sample ops with 33-byte compressed pubkeys (length 66 hex after 0x)
    const sampleOps: NodeOperationInput[] = [
        {
            operationType: 0, // Add
            nickName: "demo-add-1",
            ownerPublicKey:
                "0x02aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
            dposPublicKey:
                "0x02bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
            votes: "1000",
        },
        {
            operationType: 0, // Update
            nickName: "demo-update-1",
            ownerPublicKey:
                "0x02cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
            dposPublicKey:
                "0x02dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
            votes: "2000",
        },
    ];

    const opsToSend = sampleOps.map(
        (op) => ({
            operationType: op.operationType,
            nickName: op.nickName ?? "",
            ownerPublicKey: op.ownerPublicKey,
            dposPublicKey: op.dposPublicKey ?? "0x",
            votes: op.votes !== undefined ? BigInt(op.votes) : 0n,
        })
    );

    console.log("Prepared operations:", opsToSend);

    const tx = await pool.syncNodes(opsToSend);
    console.log("tx sent:", tx.hash);
    await tx.wait();
    console.log("syncNodes executed.");
}

main().catch((error) => {
    console.error(error);
    process.exit(1);
});

