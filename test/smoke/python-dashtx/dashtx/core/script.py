from bitcointx.core.script import (
    CScript, ScriptCoinClassDispatcher, ScriptCoinClass
)


class ScriptDashClassDispatcher(ScriptCoinClassDispatcher):
    ...


class ScriptDashClass(ScriptCoinClass,
                          metaclass=ScriptDashClassDispatcher):
    ...


class CDashScript(CScript, ScriptDashClass):
    ...


__all__ = (
    'CDashScript',
)
