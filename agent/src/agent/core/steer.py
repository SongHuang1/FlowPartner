from __future__ import annotations

import asyncio
from dataclasses import dataclass, field
from typing import Any


@dataclass
class SteerInput:
    """用户插话输入。"""
    content: str
    thread_id: str = ""
    turn_id: str = ""


class SteerQueue:
    def __init__(self) -> None:
        self._queue: asyncio.Queue[SteerInput] = asyncio.Queue(maxsize=self.MAX_SIZE)

    def push(self, steer: SteerInput) -> None:
        if self._queue.full():
            try:
                self._queue.get_nowait()
            except asyncio.QueueEmpty:
                pass
        try:
            self._queue.put_nowait(steer)
        except asyncio.QueueFull:
            pass

    def drain(self) -> list[SteerInput]:
        items: list[SteerInput] = []
        while True:
            try:
                items.append(self._queue.get_nowait())
            except asyncio.QueueEmpty:
                break
        return items

    def empty(self) -> bool:
        return self._queue.empty()

    def clear(self) -> None:
        """清空队列（中断时使用）。"""
        while not self._queue.empty():
            try:
                self._queue.get_nowait()
            except asyncio.QueueEmpty:
                break


@dataclass
class TurnContext:
    thread_id: str
    turn_id: str
    steer_queue: SteerQueue = field(default_factory=SteerQueue)
    interrupt_event: asyncio.Event = field(default_factory=asyncio.Event)

    def interrupt(self) -> None:
        self.interrupt_event.set()
        self.steer_queue.clear()

    def is_interrupted(self) -> bool:
        return self.interrupt_event.is_set()

    async def wait_interrupt(self, timeout: float) -> bool:
        try:
            await asyncio.wait_for(self.interrupt_event.wait(), timeout=timeout)
            return True
        except asyncio.TimeoutError:
            return False
