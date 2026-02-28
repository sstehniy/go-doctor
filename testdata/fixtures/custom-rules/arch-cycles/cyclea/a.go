package cyclea // want arch/forbidden-package-cycles

import "customrules/archcycles/cycleb"

func A() string { return cycleb.B() }
