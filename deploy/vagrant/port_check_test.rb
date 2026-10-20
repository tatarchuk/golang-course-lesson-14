# Checks Vagrant's host-port probe in both directions.
# Usage (repo root): /opt/vagrant/embedded/bin/ruby -I/opt/vagrant/embedded/gems/gems/vagrant-2.4.9/lib deploy/vagrant/port_check_test.rb [deploy/vagrant/port_check_fix.rb]
require "socket"
require "vagrant/util/is_port_open"
require File.expand_path(ARGV[0]) if ARGV[0]

closed = Vagrant::Util::IsPortOpen.is_port_open?("127.0.0.1", 9)

server = TCPServer.new("127.0.0.1", 0)
port = server.addr[1]
listening = Vagrant::Util::IsPortOpen.is_port_open?("127.0.0.1", port)
server.close

puts "closed port 9       -> #{closed ? 'in use (WRONG)' : 'free (right)'}"
puts "listening port #{port} -> #{listening ? 'in use (right)' : 'free (WRONG)'}"
exit(!closed && listening ? 0 : 1)
