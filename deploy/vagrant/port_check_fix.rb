# Workaround for `vagrant up` on macOS 27: Vagrant 2.4.9 decides whether a host port is taken
# with Socket.tcp(host, port, connect_timeout: 0.1). On macOS 27 that non-blocking connect reports
# a refused connection as a success, so every forwarded port (even SSH's 2222) looks "in use" and
# `vagrant up` stops with ForwardPortCollision. A plain blocking connect still gets ECONNREFUSED,
# so the probe below uses it, bounded by a one-second timeout.
# Reproduce: /opt/vagrant/embedded/bin/ruby -rsocket -e 'Socket.tcp("127.0.0.1", 9, connect_timeout: 0.1); puts "bug"'
require "socket"
require "timeout"
require "vagrant/util/is_port_open"

module Vagrant
  module Util
    module IsPortOpen
      def is_port_open?(host, port)
        Timeout.timeout(1) { TCPSocket.new(host, port).close }
        true
      rescue Timeout::Error, Errno::ETIMEDOUT, Errno::ECONNREFUSED, Errno::EHOSTUNREACH,
             Errno::ENETUNREACH, Errno::EACCES, Errno::ENOTCONN, Errno::EALREADY,
             Errno::EADDRNOTAVAIL
        false
      end
    end
  end
end
