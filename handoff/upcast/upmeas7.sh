#!/bin/bash
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
P=$S/proto
{
flock $S/gate.lock bash -c "python3 $P/j2mem.py bundle up6=$P/m_up6 up7=$P/m_up7; python3 $P/fb.py bundle 3 upE=$P/m_tg_erased up6=$P/m_up6 up7=$P/m_up7"
flock $S/gate.lock bash -c "python3 $P/j2mem.py typed-full up6=$P/m_up6 up7=$P/m_up7; python3 $P/fb.py typed-full 3 upE=$P/m_tg_erased up4=$P/m_up4 up5=$P/m_up5 up6=$P/m_up6 up7=$P/m_up7"
echo DONE
} > $S/upmeas_up7.log 2>&1
