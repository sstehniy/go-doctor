package cycleb // want arch/forbidden-package-cycles

import "customrules/archcycles/cyclea"

func B() string { return cyclea.A() }
