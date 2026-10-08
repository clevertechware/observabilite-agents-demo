package fr.clevertechware.demo.order;

import static org.assertj.core.api.Assertions.assertThat;

import jakarta.servlet.ServletException;
import java.io.IOException;
import org.junit.jupiter.api.Test;
import org.springframework.mock.web.MockFilterChain;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.mock.web.MockHttpServletResponse;

class HeapLeakFilterTest {

    private final HeapLeakFault fault = new HeapLeakFault(100);
    private final HeapLeakFilter filter = new HeapLeakFilter(fault);

    @Test
    void retains_bytes_on_business_requests() throws ServletException, IOException {
        filter.doFilter(new MockHttpServletRequest("POST", "/orders"), new MockHttpServletResponse(),
                new MockFilterChain());

        assertThat(fault.retainedBytes()).isEqualTo(100);
    }

    @Test
    void ignores_health_probes() throws ServletException, IOException {
        filter.doFilter(new MockHttpServletRequest("GET", "/healthz"), new MockHttpServletResponse(),
                new MockFilterChain());

        assertThat(fault.retainedBytes()).isZero();
    }
}
