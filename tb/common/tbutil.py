from __future__ import annotations

import cocotb
from cocotb.clock import Clock
from cocotb.triggers import FallingEdge, RisingEdge, Timer

CLK_NS = 20

def start_clock(dut, period_ns: int = CLK_NS):
    return cocotb.start_soon(Clock(dut.clk, period_ns, units="ns").start())

async def tick(dut, n: int = 1):
    for _ in range(n):
        await FallingEdge(dut.clk)

async def reset(dut, n: int = 4):
    dut.rst.value = 1
    await tick(dut, n)
    dut.rst.value = 0
    await tick(dut, 1)

async def settle():
    await Timer(1, units="ns")

def val(sig) -> int:
    return int(sig.value)
