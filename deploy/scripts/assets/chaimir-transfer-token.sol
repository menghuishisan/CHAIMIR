// 本合约是平台受控的 EVM 事件联动材料,用于真实产生标准 Transfer 日志。
// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;

contract ChaimirTransferToken {
    string public constant name = "Chaimir Token";
    string public constant symbol = "CHM";
    uint8 public constant decimals = 18;
    mapping(address => uint256) public balanceOf;

    event Transfer(address indexed from, address indexed to, uint256 value);

    constructor() {
        balanceOf[msg.sender] = 1000 ether;
    }

    function transfer(address to, uint256 value) external returns (bool) {
        require(balanceOf[msg.sender] >= value, "balance");
        balanceOf[msg.sender] -= value;
        balanceOf[to] += value;
        emit Transfer(msg.sender, to, value);
        return true;
    }
}
