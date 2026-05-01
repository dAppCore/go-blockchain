// Copyright (c) 2017-2026 Lethean (https://lt.hn)
//
// Licensed under the European Union Public Licence (EUPL) version 1.2.
// SPDX-License-Identifier: EUPL-1.2

#pragma once

#include <cstddef>
#include <cstdint>

namespace boost {
namespace multiprecision {

using limb_type = std::uint64_t;

enum cpp_integer_type {
	signed_magnitude,
	unsigned_magnitude,
};

enum cpp_int_check_type {
	unchecked,
	checked,
};

enum expression_template_option {
	et_off,
	et_on,
};

template <unsigned MinBits = 0, unsigned MaxBits = 0,
	cpp_integer_type SignType = signed_magnitude,
	cpp_int_check_type Checked = unchecked,
	class Allocator = void>
class cpp_int_backend {};

template <class Backend, expression_template_option ExpressionTemplates = et_off>
class number {
public:
	number() = default;
	number(unsigned long long) {}

	class backend_type {
	public:
		std::size_t size() const { return 0; }
		static constexpr std::size_t limb_bits = sizeof(limb_type) * 8;

		limb_type *limbs() { return nullptr; }
		const limb_type *limbs() const { return nullptr; }

		void resize(unsigned, unsigned) {}
		void normalize() {}
	};

	backend_type &backend() { return backend_; }
	const backend_type &backend() const { return backend_; }

private:
	backend_type backend_{};
};

using uint128_t = number<cpp_int_backend<128, 128, unsigned_magnitude, unchecked, void>>;
using uint256_t = number<cpp_int_backend<256, 256, unsigned_magnitude, unchecked, void>>;
using uint512_t = number<cpp_int_backend<512, 512, unsigned_magnitude, unchecked, void>>;

} // namespace multiprecision
} // namespace boost
