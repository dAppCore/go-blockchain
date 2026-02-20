// SPDX-Licence-Identifier: EUPL-1.2
// Compat stub for CryptoNote warning macros.
// Replaces contrib/epee/include/warnings.h without Boost dependency.
// Uses a double-stringify trick to replicate BOOST_PP_STRINGIZE.
#pragma once

#define _CN_STRINGIFY_INNER(x) #x
#define _CN_STRINGIFY(x) _CN_STRINGIFY_INNER(x)

#if defined(_MSC_VER)
#define PUSH_VS_WARNINGS    __pragma(warning(push))
#define POP_VS_WARNINGS     __pragma(warning(pop))
#define DISABLE_VS_WARNINGS(w) __pragma(warning(disable: w))
#define PUSH_GCC_WARNINGS
#define POP_GCC_WARNINGS
#define DISABLE_GCC_WARNING(w)
#define DISABLE_CLANG_WARNING(w)
#define DISABLE_GCC_AND_CLANG_WARNING(w)
#define ATTRIBUTE_UNUSED
#else
#define PUSH_VS_WARNINGS
#define POP_VS_WARNINGS
#define DISABLE_VS_WARNINGS(w)
#define PUSH_GCC_WARNINGS   _Pragma("GCC diagnostic push")
#define POP_GCC_WARNINGS    _Pragma("GCC diagnostic pop")
#define ATTRIBUTE_UNUSED __attribute__((unused))

#define DISABLE_GCC_AND_CLANG_WARNING(w) \
    _Pragma(_CN_STRINGIFY(GCC diagnostic ignored _CN_STRINGIFY(-W##w)))

#if defined(__clang__)
#define DISABLE_GCC_WARNING(w)
#define DISABLE_CLANG_WARNING DISABLE_GCC_AND_CLANG_WARNING
#else
#define DISABLE_GCC_WARNING DISABLE_GCC_AND_CLANG_WARNING
#define DISABLE_CLANG_WARNING(w)
#endif

#endif
