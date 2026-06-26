include "root" {
  path = find_in_parent_folders()
}

terraform {
  source = "${dirname(find_in_parent_folders())}//stack"
}

# Ambiente DEV: menor footprint possível, par único (matching mais leve).
inputs = {
  env                = "dev"
  image_tag          = "latest"
  pairs              = "VLT/USDT-sim"
  rds_instance_class = "db.t4g.micro"
  mq_instance_type   = "mq.t3.micro"
  enable_nat         = false
}
