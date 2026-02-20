// SPDX-Licence-Identifier: EUPL-1.2
// Compat stub for epee logging, exception handling, and misc macros.
#pragma once

#include <cassert>
#include <iostream>

#define ENDL std::endl

#define LOG_PRINT_RED(msg, level) ((void)0)
#define LOG_PRINT_L0(msg) ((void)0)
#define LOG_PRINT_L1(msg) ((void)0)
#define LOG_PRINT_L2(msg) ((void)0)
#define LOG_PRINT_L3(msg) ((void)0)
#define LOG_PRINT_L4(msg) ((void)0)
#define LOG_ERROR(msg) ((void)0)

#define CHECK_AND_ASSERT_MES(cond, ret, msg) \
  do { if (!(cond)) { return (ret); } } while(0)

#define CHECK_AND_ASSERT_MES_NO_RET(cond, msg) \
  do { if (!(cond)) { return; } } while(0)

#define CHECK_AND_NO_ASSERT_MES(cond, ret, msg) \
  do { if (!(cond)) { return (ret); } } while(0)

#define ASSERT_MES_AND_THROW(msg) do { assert(false && (msg)); } while(0)

// Exception handling macros (from misc_helpers.h)
#define TRY_ENTRY() try {
#define CATCH_ENTRY(location, return_val) } catch(...) { return (return_val); }
#define CATCH_ENTRY2(return_val) } catch(...) { return (return_val); }
#define CATCH_ENTRY_CUSTOM(location, custom_code, return_val) } catch(...) { custom_code; return (return_val); }
#define CATCH_ENTRY_CUSTOM2(custom_code, return_val) } catch(...) { custom_code; return (return_val); }
#define CATCH_ENTRY_L0(location, return_val) CATCH_ENTRY(location, return_val)
#define CATCH_ENTRY_L1(location, return_val) CATCH_ENTRY(location, return_val)

// Location string helper
#define LOCATION_SS ""
