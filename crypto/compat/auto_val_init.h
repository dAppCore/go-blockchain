// SPDX-Licence-Identifier: EUPL-1.2
// Compat stub for auto_val_init.h — replaces boost::value_initialized with
// aggregate zero-initialisation which is equivalent for POD types.
#pragma once

#define AUTO_VAL_INIT(v)   decltype(v){}
#define AUTO_VAL_INIT_T(t) t{}
