package fr.clevertechware.demo.order;

import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;

class HeapLeakFaultTest {

    @Test
    void retains_configured_bytes_for_each_request() {
        HeapLeakFault fault = new HeapLeakFault(1024);

        fault.onRequest();
        fault.onRequest();
        fault.onRequest();

        assertThat(fault.retainedBytes()).isEqualTo(3 * 1024);
    }

    @Test
    void retains_nothing_when_bytes_per_request_is_zero() {
        HeapLeakFault fault = new HeapLeakFault(0);

        fault.onRequest();

        assertThat(fault.retainedBytes()).isZero();
    }
}
